package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// DeployStore handles deployment and container-row persistence.
type DeployStore struct {
	pool *pgxpool.Pool
}

// NewDeployStore returns a DeployStore bound to the given pool.
func NewDeployStore(pool *pgxpool.Pool) *DeployStore {
	return &DeployStore{pool: pool}
}

// CreateDeployment inserts a new deployment row, assigning the next per-app
// version and returning the fully populated row. It clears is_current on prior
// deployments in the same transaction so only one is ever "current".
func (s *DeployStore) CreateDeployment(ctx context.Context, appID, imageTag string) (*models.Deployment, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx) // safe to call after commit; no-op then

	// Clear any existing current marker for this app.
	if _, err := tx.Exec(ctx, `UPDATE deployments SET is_current = FALSE WHERE app_id = $1`, appID); err != nil {
		return nil, fmt.Errorf("clear current: %w", err)
	}

	d := &models.Deployment{AppID: appID, ImageTag: imageTag, Status: models.DeploymentStatusDeploying}
	const q = `
		INSERT INTO deployments (app_id, image_tag, status, started_at)
		VALUES ($1, $2, $3, NOW())
		RETURNING id, version, is_current, started_at, created_at`
	err = tx.QueryRow(ctx, q, appID, imageTag, d.Status).Scan(
		&d.ID, &d.Version, &d.IsCurrent, &d.StartedAt, &d.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert deployment: %w", mapPgErr(err))
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return d, nil
}

// MarkLive marks the deployment current and live, completing it.
func (s *DeployStore) MarkLive(ctx context.Context, deploymentID string) error {
	const q = `
		UPDATE deployments SET
			is_current   = TRUE,
			status       = $2,
			completed_at = NOW()
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, deploymentID, models.DeploymentStatusLive)
	return mapPgErr(err)
}

// MarkFailed records a failure reason and completion time.
func (s *DeployStore) MarkFailed(ctx context.Context, deploymentID, errMsg string) error {
	const q = `
		UPDATE deployments SET
			status       = $2,
			error_message = $3,
			completed_at  = NOW()
		WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, deploymentID, models.DeploymentStatusFailed, errMsg)
	return mapPgErr(err)
}

// ListDeployments returns all deployments for an app, newest version first.
func (s *DeployStore) ListDeployments(ctx context.Context, appID string) ([]*models.Deployment, error) {
	const q = `
		SELECT id, app_id, version, image_tag, status, error_message,
		       is_current, started_at, completed_at, created_at
		FROM deployments WHERE app_id = $1 ORDER BY version DESC`
	rows, err := s.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("list deployments: %w", err)
	}
	defer rows.Close()

	var deps []*models.Deployment
	for rows.Next() {
		d := &models.Deployment{}
		if err := rows.Scan(
			&d.ID, &d.AppID, &d.Version, &d.ImageTag, &d.Status, &d.ErrorMessage,
			&d.IsCurrent, &d.StartedAt, &d.CompletedAt, &d.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deployment: %w", err)
		}
		deps = append(deps, d)
	}
	return deps, rows.Err()
}

// --- Containers -----------------------------------------------------------

// CreateContainer inserts a tracked-container row.
func (s *DeployStore) CreateContainer(ctx context.Context, c *models.Container) error {
	const q = `
		INSERT INTO containers (id, app_id, deployment_id, container_id, name, status, port_mapping, started_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
		RETURNING created_at`
	return mapPgErr(s.pool.QueryRow(ctx, q,
		c.ID, c.AppID, c.DeploymentID, c.ContainerID, c.Name, c.Status, c.PortMapping,
	).Scan(&c.CreatedAt))
}

// ListRunningContainersByApp returns containers still tracked as running for an app.
func (s *DeployStore) ListRunningContainersByApp(ctx context.Context, appID string) ([]*models.Container, error) {
	const q = `
		SELECT id, app_id, deployment_id, container_id, name, status, port_mapping,
		       started_at, stopped_at, restart_count, created_at
		FROM containers
		WHERE app_id = $1 AND status IN ('starting','running')
		ORDER BY created_at`
	return scanContainers(s.pool.Query(ctx, q, appID))
}

// ListContainersByApp returns all tracked containers for an app.
func (s *DeployStore) ListContainersByApp(ctx context.Context, appID string) ([]*models.Container, error) {
	const q = `
		SELECT id, app_id, deployment_id, container_id, name, status, port_mapping,
		       started_at, stopped_at, restart_count, created_at
		FROM containers WHERE app_id = $1 ORDER BY created_at DESC`
	return scanContainers(s.pool.Query(ctx, q, appID))
}

// SetContainerStatus updates a container row's status and stopped_at if needed.
func (s *DeployStore) SetContainerStatus(ctx context.Context, id, status string) error {
	q := `UPDATE containers SET status = $2`
	args := []interface{}{id, status}
	if status == models.ContainerStatusStopped || status == models.ContainerStatusCrashed {
		q += `, stopped_at = NOW()`
	}
	q += ` WHERE id = $1`
	_, err := s.pool.Exec(ctx, q, args...)
	return mapPgErr(err)
}

// DeleteContainer removes a tracked-container row.
func (s *DeployStore) DeleteContainer(ctx context.Context, id string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM containers WHERE id = $1`, id)
	return mapPgErr(err)
}

// scanContainers drains rows into a slice of container models.
func scanContainers(rows pgx.Rows, err error) ([]*models.Container, error) {
	if err != nil {
		return nil, fmt.Errorf("query containers: %w", err)
	}
	defer rows.Close()

	var out []*models.Container
	for rows.Next() {
		c := &models.Container{}
		if err := rows.Scan(
			&c.ID, &c.AppID, &c.DeploymentID, &c.ContainerID, &c.Name, &c.Status, &c.PortMapping,
			&c.StartedAt, &c.StoppedAt, &c.RestartCount, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan container: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
