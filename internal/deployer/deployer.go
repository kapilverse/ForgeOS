package deployer

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"

	"forgeos/internal/builder"
	"forgeos/internal/container"
	"forgeos/internal/models"
	"forgeos/internal/router"
	"forgeos/internal/store"
)

// readinessTimeout is how long we wait for a started container to report
// "running" before declaring the deployment failed.
const readinessTimeout = 60 * time.Second

// Deployer coordinates the container manager, Traefik label generator, and DB.
type Deployer struct {
	containers *container.Manager
	router     *router.Traefik
	store      *store.Store
	builder    *builder.Builder
	logger     *log.Logger
}

// New returns a wired Deployer.
func New(containers *container.Manager, rt *router.Traefik, st *store.Store, b *builder.Builder, logger *log.Logger) *Deployer {
	if logger == nil {
		logger = log.Default()
	}
	return &Deployer{containers: containers, router: rt, store: st, builder: b, logger: logger}
}

// BuildAndDeploy creates a deployment and build record, then launches a background
// goroutine to clone, build the image, and roll it out. It returns immediately
// with the pending deployment.
func (d *Deployer) BuildAndDeploy(ctx context.Context, app *models.App, repoURL, branch string) (*models.Deployment, error) {
	if repoURL == "" {
		return nil, errors.New("repo URL is required for source deployments")
	}

	deployment, err := d.store.Deploy.CreateDeployment(ctx, app.ID, "") // ImageTag populated after build
	if err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	b := &models.Build{
		ID:           uuid.NewString(),
		DeploymentID: deployment.ID,
		Status:       models.BuildStatusPending,
	}
	if err := d.store.Builds.Create(ctx, b); err != nil {
		return nil, fmt.Errorf("create build record: %w", err)
	}

	// Update app status to indicate work is happening
	_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusDeploying)

	// Launch async build
	go d.runBuildAndRollout(context.Background(), app, deployment, repoURL, branch)

	return deployment, nil
}

func (d *Deployer) runBuildAndRollout(ctx context.Context, app *models.App, deployment *models.Deployment, repoURL, branch string) {
	_ = d.store.Builds.SetStatus(ctx, deployment.ID, models.BuildStatusRunning)

	req := builder.BuildRequest{
		AppSlug: app.Slug,
		Version: deployment.Version,
		RepoURL: repoURL,
		Branch:  branch,
		Port:    app.Port,
		LogSink: func(line string) {
			_ = d.store.Builds.AppendLog(ctx, deployment.ID, line)
		},
	}

	imageTag, err := d.builder.Build(ctx, req)
	if err != nil {
		_ = d.store.Builds.AppendLog(ctx, deployment.ID, fmt.Sprintf("Build failed: %v\n", err))
		_ = d.store.Builds.SetStatus(ctx, deployment.ID, models.BuildStatusFailed)
		_ = d.store.Deploy.MarkFailed(ctx, deployment.ID, err.Error())
		_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusError)
		return
	}

	_ = d.store.Builds.SetStatus(ctx, deployment.ID, models.BuildStatusSuccess)

	// Update deployment with the built image tag
	_ = d.store.Deploy.UpdateImage(ctx, deployment.ID, imageTag)

	// Proceed to rollout
	if err := d.rollout(ctx, app, deployment, imageTag); err != nil {
		d.logger.Printf("rollout failed for app=%s: %v", app.Slug, err)
	}
}

// DeployImage rolls out an image for an app. It returns the created deployment.
// The app is expected to be already loaded and authorized by the caller.
func (d *Deployer) DeployImage(ctx context.Context, app *models.App, imageRef string) (*models.Deployment, error) {
	if imageRef == "" {
		return nil, errors.New("image reference is required")
	}

	deployment, err := d.store.Deploy.CreateDeployment(ctx, app.ID, imageRef)
	if err != nil {
		return nil, fmt.Errorf("create deployment: %w", err)
	}

	d.logger.Printf("deploying app=%s version=%d image=%s", app.Slug, deployment.Version, imageRef)

	// Best-effort: mark the app as deploying so the UI reflects in-progress work.
	_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusDeploying)

	err = d.rollout(ctx, app, deployment, imageRef)
	if err != nil {
		return deployment, err
	}

	return deployment, nil
}

func (d *Deployer) rollout(ctx context.Context, app *models.App, deployment *models.Deployment, imageRef string) error {
	// 1. Snapshot the old rows so we can remove them later
	oldRows, _ := d.store.Deploy.ListRunningContainersByApp(ctx, app.ID)

	// 2. Start the new replicas alongside the old ones
	created, err := d.startReplicas(ctx, app, deployment.ID, imageRef)
	if err != nil {
		d.cleanupCreated(ctx, created)
		_ = d.store.Deploy.MarkFailed(ctx, deployment.ID, err.Error())
		_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusError)
		return fmt.Errorf("start replicas: %w", err)
	}

	// 3. Wait for new replicas to become running and pass health checks
	for _, c := range created {
		if err := d.containers.WaitForRunning(ctx, c.ID, readinessTimeout); err != nil {
			d.cleanupCreated(ctx, created)
			_ = d.store.Deploy.MarkFailed(ctx, deployment.ID, fmt.Sprintf("replica %s not running: %v", c.Name, err))
			_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusError)
			return fmt.Errorf("replica readiness: %w", err)
		}

		if app.HealthCheck != "" && c.HostPort > 0 {
			if err := WaitForHealthy(ctx, c.HostPort, app.HealthCheck, readinessTimeout); err != nil {
				d.cleanupCreated(ctx, created)
				_ = d.store.Deploy.MarkFailed(ctx, deployment.ID, fmt.Sprintf("replica %s health check failed: %v", c.Name, err))
				_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusError)
				return fmt.Errorf("replica health: %w", err)
			}
		}
	}

	// 4. Drain period (wait for Traefik to switch over and finish active requests to old ones)
	if len(oldRows) > 0 {
		time.Sleep(10 * time.Second)
	}

	// 5. Cleanup old containers
	keepIDs := make([]string, len(created))
	for i, c := range created {
		keepIDs[i] = c.ID
	}
	if errs := d.containers.RemoveByAppExcept(ctx, app.Slug, keepIDs); len(errs) > 0 {
		for _, e := range errs {
			d.logger.Printf("warn: cleanup old containers app=%s: %v", app.Slug, e)
		}
	}
	for _, c := range oldRows {
		_ = d.store.Deploy.SetContainerStatus(ctx, c.ID, models.ContainerStatusStopped)
	}

	// 6. Persist the new container rows and flip the deployment + app to live.
	now := time.Now()
	for i := range created {
		c := created[i]
		row := &models.Container{
			ID: uuid.NewString(), AppID: app.ID, DeploymentID: deployment.ID,
			ContainerID: c.ID, Name: c.Name, Status: models.ContainerStatusRunning,
			PortMapping: c.HostPort, StartedAt: &now,
		}
		if err := d.store.Deploy.CreateContainer(ctx, row); err != nil {
			d.logger.Printf("warn: record container row app=%s name=%s: %v", app.Slug, c.Name, err)
		}
	}

	if err := d.store.Deploy.MarkLive(ctx, deployment.ID); err != nil {
		d.logger.Printf("warn: mark deployment live app=%s: %v", app.Slug, err)
	}
	if err := d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusLive); err != nil {
		d.logger.Printf("warn: set app live app=%s: %v", app.Slug, err)
	}

	d.logger.Printf("deploy complete app=%s version=%d replicas=%d", app.Slug, deployment.Version, len(created))
	return nil
}

// startReplicas brings up the requested number of containers for the app and
// returns the high-level container views (already started). On partial failure
// it returns the containers created so far plus the error so the caller can
// clean them up.
func (d *Deployer) startReplicas(ctx context.Context, app *models.App, deploymentID, imageRef string) ([]*container.Container, error) {
	replicas := app.Replicas
	if replicas < 1 {
		replicas = 1
	}
	created := make([]*container.Container, 0, replicas)
	for i := 0; i < replicas; i++ {
		labels := d.router.Labels(router.LabelConfig{
			Slug:        app.Slug, 
			Port:        app.Port, 
			Replica:     i,
			HealthCheck: app.HealthCheck,
		})
		c, err := d.containers.Create(ctx, container.ContainerConfig{
			AppID:       app.Slug,
			Image:       imageRef,
			Port:        app.Port,
			EnvVars:     nil, // env-var injection arrives with the env-var sprint
			MemoryLimit: app.MemoryLimit,
			CPUShares:   app.CPULimit,
			Labels:      labels,
		})
		if err != nil {
			return created, fmt.Errorf("replica %d: %w", i, err)
		}
		created = append(created, c)
	}
	return created, nil
}

// cleanupCreated stops and removes a set of containers, ignoring per-container
// errors so one failure doesn't abort cleanup of the rest.
func (d *Deployer) cleanupCreated(ctx context.Context, created []*container.Container) {
	for _, c := range created {
		if err := d.containers.Stop(ctx, c.ID, 5); err != nil {
			d.logger.Printf("warn: stop during cleanup name=%s: %v", c.Name, err)
		}
		if err := d.containers.Remove(ctx, c.ID); err != nil {
			d.logger.Printf("warn: remove during cleanup name=%s: %v", c.Name, err)
		}
	}
}

// StopApp gracefully stops all running replicas of an app and updates DB state.
func (d *Deployer) StopApp(ctx context.Context, app *models.App) error {
	rows, err := d.store.Deploy.ListRunningContainersByApp(ctx, app.ID)
	if err != nil {
		return fmt.Errorf("list running containers: %w", err)
	}
	var firstErr error
	for _, c := range rows {
		if err := d.containers.Stop(ctx, c.ContainerID, 10); err != nil && firstErr == nil {
			firstErr = err
		}
		_ = d.containers.Remove(ctx, c.ContainerID)
		_ = d.store.Deploy.SetContainerStatus(ctx, c.ID, models.ContainerStatusStopped)
	}
	_ = d.store.Apps.SetStatus(ctx, app.UserID, app.ID, models.AppStatusStopped)
	return firstErr
}

// StopAppWithCtx is the server.AppLifecycle adapter for StopApp.
func (d *Deployer) StopAppWithCtx(ctx context.Context, app *models.App) error {
	return d.StopApp(ctx, app)
}

// DeleteApp removes all containers and (via the store) the app row + children.
func (d *Deployer) DeleteApp(ctx context.Context, app *models.App) error {
	if errs := d.containers.RemoveByApp(ctx, app.Slug); len(errs) > 0 {
		for _, e := range errs {
			d.logger.Printf("warn: remove containers app=%s: %v", app.Slug, e)
		}
	}
	return d.store.Apps.Delete(ctx, app.UserID, app.ID)
}

// DeleteAppWithCtx is the server.AppLifecycle adapter for DeleteApp.
func (d *Deployer) DeleteAppWithCtx(ctx context.Context, app *models.App) error {
	return d.DeleteApp(ctx, app)
}
