// Package container wraps the Docker SDK to manage the lifecycle of ForgeOS
// app containers: create (with resource limits, env injection, network attach,
// and Traefik routing labels), start, stop (graceful), remove, and inspect.
package container

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	"github.com/google/uuid"

	"forgeos/internal/models"
)

// Manager owns the Docker client and high-level container operations.
type Manager struct {
	cli     *client.Client
	network string // the shared bridge network app containers + Traefik join
}

// New creates a Manager bound to the local Docker daemon and the given network.
func New(ctx context.Context, dockerNetwork string) (*Manager, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	if _, err := cli.Ping(ctx); err != nil {
		return nil, fmt.Errorf("docker ping: %w", err)
	}
	return &Manager{cli: cli, network: dockerNetwork}, nil
}

// Client exposes the underlying Docker client for packages that need
// operations not yet wrapped (e.g. the router querying containers).
func (m *Manager) Client() *client.Client { return m.cli }

// ContainerConfig fully describes a replica to be created.
type ContainerConfig struct {
	AppID       string            // app slug (used in container name + labels)
	Image       string            // image:tag to run
	Port        int               // container port the app listens on
	EnvVars     map[string]string // injected as container env
	MemoryLimit int               // MB
	CPUShares   int               // CPU shares (relative weight)
	Labels      map[string]string // Docker labels (Traefik routing, metadata)
}

// Container is the high-level view returned after creating a replica.
type Container struct {
	ID       string // Docker container ID
	Name     string
	HostPort int // host port mapped (0 if none)
}

// Create pulls the image (if needed), creates and starts a container, attaches
// it to the shared network, and returns its id/name/mapped port.
func (m *Manager) Create(ctx context.Context, cfg ContainerConfig) (*Container, error) {
	if err := m.pullImage(ctx, cfg.Image); err != nil {
		return nil, fmt.Errorf("pull image: %w", err)
	}

	name := fmt.Sprintf("forgeos-%s-%s", cfg.AppID, shortID())

	// env as ["KEY=value", ...]
	env := make([]string, 0, len(cfg.EnvVars))
	for k, v := range cfg.EnvVars {
		env = append(env, k+"="+v)
	}

	// Metadata labels so ForgeOS can find its own containers later.
	if cfg.Labels == nil {
		cfg.Labels = map[string]string{}
	}
	cfg.Labels["forgeos.managed"] = "true"
	cfg.Labels["forgeos.app"] = cfg.AppID

	// Resources: memory in bytes, CPU as relative shares.
	resources := container.Resources{}
	if cfg.MemoryLimit > 0 {
		resources.Memory = int64(cfg.MemoryLimit) * 1024 * 1024
	}
	if cfg.CPUShares > 0 {
		resources.CPUShares = int64(cfg.CPUShares)
	}

	// Port binding to localhost (auto-assigned) for health checks.
	portMap := nat.PortMap{
		nat.Port(fmt.Sprintf("%d/tcp", cfg.Port)): []nat.PortBinding{
			{HostIP: "127.0.0.1", HostPort: "0"},
		},
	}

	createResp, err := m.cli.ContainerCreate(ctx,
		&container.Config{
			Image:  cfg.Image,
			Env:    env,
			Labels: cfg.Labels,
			ExposedPorts: nat.PortSet{
				nat.Port(fmt.Sprintf("%d/tcp", cfg.Port)): struct{}{},
			},
		},
		&container.HostConfig{
			Resources: resources,
			PortBindings: portMap,
			RestartPolicy: container.RestartPolicy{
				Name:              container.RestartPolicyOnFailure,
				MaximumRetryCount: 3,
			},
		},
		&network.NetworkingConfig{
			EndpointsConfig: map[string]*network.EndpointSettings{
				m.network: {},
			},
		},
		nil, name,
	)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	if err := m.cli.ContainerStart(ctx, createResp.ID, container.StartOptions{}); err != nil {
		// Best-effort cleanup of the half-created container.
		_ = m.cli.ContainerRemove(ctx, createResp.ID, container.RemoveOptions{Force: true})
		return nil, fmt.Errorf("start container: %w", err)
	}

	c := &Container{ID: createResp.ID, Name: name}
	if hp, err := m.hostPort(ctx, createResp.ID, cfg.Port); err == nil {
		c.HostPort = hp
	}
	return c, nil
}

// Stop gracefully stops a container: SIGTERM, then SIGKILL after timeout.
func (m *Manager) Stop(ctx context.Context, containerID string, timeoutSecs int) error {
	to := timeoutSecs
	if err := m.cli.ContainerStop(ctx, containerID, container.StopOptions{Timeout: &to}); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}
	return nil
}

// Remove force-removes a container and its anonymous volumes.
func (m *Manager) Remove(ctx context.Context, containerID string) error {
	if err := m.cli.ContainerRemove(ctx, containerID, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	return nil
}

// IsRunning reports whether a container exists and is currently running.
func (m *Manager) IsRunning(ctx context.Context, containerID string) (bool, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return false, fmt.Errorf("inspect container: %w", err)
	}
	return inspect.State != nil && inspect.State.Running, nil
}

// ListByApp returns the Docker container IDs ForgeOS manages for the given
// app slug, optionally filtered by running state.
func (m *Manager) ListByApp(ctx context.Context, appSlug string, runningOnly bool) ([]string, error) {
	args := filters.NewArgs()
	args.Add("label", "forgeos.managed=true")
	args.Add("label", "forgeos.app="+appSlug)
	list, err := m.cli.ContainerList(ctx, container.ListOptions{
		All:     !runningOnly, // include stopped unless caller wants live only
		Filters: args,
	})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}
	ids := make([]string, 0, len(list))
	for _, c := range list {
		if runningOnly && c.State != "running" {
			continue
		}
		ids = append(ids, c.ID)
	}
	return ids, nil
}

// RemoveByApp stops and removes every container ForgeOS manages for an app.
func (m *Manager) RemoveByApp(ctx context.Context, appSlug string) []error {
	return m.RemoveByAppExcept(ctx, appSlug, nil)
}

// RemoveByAppExcept stops and removes every container for an app EXCEPT the specified IDs.
func (m *Manager) RemoveByAppExcept(ctx context.Context, appSlug string, keepIDs []string) []error {
	ids, err := m.ListByApp(ctx, appSlug, false)
	if err != nil {
		return []error{err}
	}

	keepMap := make(map[string]bool)
	for _, id := range keepIDs {
		keepMap[id] = true
	}

	var errs []error
	for _, id := range ids {
		if keepMap[id] {
			continue
		}
		if err := m.Stop(ctx, id, 10); err != nil {
			errs = append(errs, err)
		}
		if err := m.Remove(ctx, id); err != nil {
			errs = append(errs, err)
		}
	}
	return errs
}

// WaitForRunning polls the container until it reports running or the timeout
// elapses. This is a lightweight readiness gate used in this sprint; a proper
// HTTP health-gated rollout arrives with the zero-downtime sprint.
func (m *Manager) WaitForRunning(ctx context.Context, containerID string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		ok, err := m.IsRunning(ctx, containerID)
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("container %s did not reach running state within %s", shortIDOf(containerID), timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// --- helpers --------------------------------------------------------------

// pullImage ensures the image is present locally, pulling if necessary.
func (m *Manager) pullImage(ctx context.Context, ref string) error {
	// Fast path: if it's already local, skip the (potentially slow) pull.
	if images, err := m.cli.ImageList(ctx, image.ListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", ref)),
	}); err == nil && len(images) > 0 {
		return nil
	}

	reader, err := m.cli.ImagePull(ctx, ref, image.PullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	_, _ = io.Copy(io.Discard, reader) // drain to completion
	return nil
}

// hostPort resolves the host port mapped to a container's internal port, if any.
func (m *Manager) hostPort(ctx context.Context, containerID string, port int) (int, error) {
	inspect, err := m.cli.ContainerInspect(ctx, containerID)
	if err != nil {
		return 0, err
	}
	// NetworkSettings may be nil for a freshly started container.
	if inspect.NetworkSettings == nil {
		return 0, fmt.Errorf("no network settings")
	}
	for _, bindings := range inspect.NetworkSettings.Ports {
		for _, b := range bindings {
			if b.HostPort == "" {
				continue
			}
			// We did not publish ports (traffic flows via Traefik over the
			// shared network), so return the first host port we find, if any.
			var p int
			if _, err := fmt.Sscanf(b.HostPort, "%d", &p); err == nil {
				return p, nil
			}
		}
	}
	_ = port
	return 0, fmt.Errorf("no host port published")
}

// shortID returns a short random suffix for container names.
func shortID() string {
	return strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
}

// shortIDOf trims a Docker container id for log/error messages.
func shortIDOf(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// Ensure models is referenced even if future fields use it.
var _ = models.AppStatusLive
