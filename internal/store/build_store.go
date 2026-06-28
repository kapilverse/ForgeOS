package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// BuildStore handles build-row persistence: status and the streamed build log.
type BuildStore struct {
	pool *pgxpool.Pool
}

// NewBuildStore returns a BuildStore bound to the given pool.
func NewBuildStore(pool *pgxpool.Pool) *BuildStore {
	return &BuildStore{pool: pool}
}

// Create inserts a new build row for a deployment. One build per deployment.
func (s *BuildStore) Create(ctx context.Context, b *models.Build) error {
	const q = `
		INSERT INTO builds (id, deployment_id, status, started_at)
		VALUES ($1, $2, $3, NOW())
		ON CONFLICT (deployment_id) DO NOTHING
		RETURNING created_at, started_at`
	return mapPgErr(s.pool.QueryRow(ctx, q, b.ID, b.DeploymentID, b.Status).Scan(&b.CreatedAt, &b.StartedAt))
}

// AppendLog appends a chunk to the build's log column. The build log can grow
// large during a build; callers append line-by-line as the docker build streams.
func (s *BuildStore) AppendLog(ctx context.Context, deploymentID, chunk string) error {
	const q = `UPDATE builds SET log = log || $2 WHERE deployment_id = $1`
	_, err := s.pool.Exec(ctx, q, deploymentID, chunk)
	return mapPgErr(err)
}

// SetStatus updates the build status. On terminal statuses it also records the
// completion time and duration.
func (s *BuildStore) SetStatus(ctx context.Context, deploymentID, status string) error {
	q := `UPDATE builds SET status = $2`
	if status == models.BuildStatusSuccess || status == models.BuildStatusFailed {
		q += `, completed_at = NOW(),
		       duration_ms  = EXTRACT(EPOCH FROM (NOW() - started_at)) * 1000`
	}
	q += ` WHERE deployment_id = $1`
	cmd, err := s.pool.Exec(ctx, q, deploymentID, status)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByDeployment loads the build row for a deployment, if any. Image-mode
// deployments have no build row and return ErrNotFound.
func (s *BuildStore) GetByDeployment(ctx context.Context, deploymentID string) (*models.Build, error) {
	b := &models.Build{}
	const q = `
		SELECT id, deployment_id, status, log, duration_ms, started_at, completed_at, created_at
		FROM builds WHERE deployment_id = $1`
	err := s.pool.QueryRow(ctx, q, deploymentID).Scan(
		&b.ID, &b.DeploymentID, &b.Status, &b.Log, &b.DurationMs, &b.StartedAt, &b.CompletedAt, &b.CreatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return b, nil
}

// FormatDurationMs is a tiny helper so handlers don't need to import fmt for it.
var _ = fmt.Sprintf
