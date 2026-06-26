package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// AppStore handles app persistence.
type AppStore struct {
	pool *pgxpool.Pool
}

// NewAppStore returns an AppStore bound to the given pool.
func NewAppStore(pool *pgxpool.Pool) *AppStore {
	return &AppStore{pool: pool}
}

// Create inserts a new app owned by userID.
func (s *AppStore) Create(ctx context.Context, a *models.App) error {
	const q = `
		INSERT INTO apps (id, user_id, name, slug, description, docker_image,
		                  status, replicas, cpu_limit, memory_limit, port, health_check)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		RETURNING created_at, updated_at`
	err := s.pool.QueryRow(ctx, q,
		a.ID, a.UserID, a.Name, a.Slug, a.Description, a.DockerImage,
		a.Status, a.Replicas, a.CPULimit, a.MemoryLimit, a.Port, a.HealthCheck,
	).Scan(&a.CreatedAt, &a.UpdatedAt)
	return mapPgErr(err)
}

// GetByID loads an app, scoped to userID so a user can never read another
// user's app. Returns ErrNotFound if the app doesn't exist or is owned by
// someone else.
func (s *AppStore) GetByID(ctx context.Context, userID, id string) (*models.App, error) {
	a := &models.App{}
	const q = `
		SELECT id, user_id, name, slug, description, docker_image, status,
		       replicas, cpu_limit, memory_limit, port, health_check, created_at, updated_at
		FROM apps WHERE id = $1 AND user_id = $2`
	err := s.pool.QueryRow(ctx, q, id, userID).Scan(
		&a.ID, &a.UserID, &a.Name, &a.Slug, &a.Description, &a.DockerImage, &a.Status,
		&a.Replicas, &a.CPULimit, &a.MemoryLimit, &a.Port, &a.HealthCheck, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return a, nil
}

// GetBySlug loads an app by its URL slug (globally unique).
func (s *AppStore) GetBySlug(ctx context.Context, slug string) (*models.App, error) {
	a := &models.App{}
	const q = `
		SELECT id, user_id, name, slug, description, docker_image, status,
		       replicas, cpu_limit, memory_limit, port, health_check, created_at, updated_at
		FROM apps WHERE slug = $1`
	err := s.pool.QueryRow(ctx, q, slug).Scan(
		&a.ID, &a.UserID, &a.Name, &a.Slug, &a.Description, &a.DockerImage, &a.Status,
		&a.Replicas, &a.CPULimit, &a.MemoryLimit, &a.Port, &a.HealthCheck, &a.CreatedAt, &a.UpdatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return a, nil
}

// ListByUser returns all apps owned by userID, newest first.
func (s *AppStore) ListByUser(ctx context.Context, userID string) ([]*models.App, error) {
	const q = `
		SELECT id, user_id, name, slug, description, docker_image, status,
		       replicas, cpu_limit, memory_limit, port, health_check, created_at, updated_at
		FROM apps WHERE user_id = $1 ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list apps: %w", err)
	}
	defer rows.Close()

	var apps []*models.App
	for rows.Next() {
		a := &models.App{}
		if err := rows.Scan(
			&a.ID, &a.UserID, &a.Name, &a.Slug, &a.Description, &a.DockerImage, &a.Status,
			&a.Replicas, &a.CPULimit, &a.MemoryLimit, &a.Port, &a.HealthCheck, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan app: %w", err)
		}
		apps = append(apps, a)
	}
	return apps, rows.Err()
}

// Update applies mutable app fields (description, resources, port, health).
func (s *AppStore) Update(ctx context.Context, a *models.App) error {
	const q = `
		UPDATE apps SET
			description  = $2,
			cpu_limit    = $3,
			memory_limit = $4,
			port         = $5,
			health_check = $6,
			updated_at   = NOW()
		WHERE id = $1 AND user_id = $7
		RETURNING updated_at`
	err := s.pool.QueryRow(ctx, q,
		a.ID, a.Description, a.CPULimit, a.MemoryLimit, a.Port, a.HealthCheck, a.UserID,
	).Scan(&a.UpdatedAt)
	return mapPgErr(err)
}

// SetStatus updates only the lifecycle status of an app.
func (s *AppStore) SetStatus(ctx context.Context, userID, id, status string) error {
	const q = `UPDATE apps SET status = $3, updated_at = NOW() WHERE id = $1 AND user_id = $2`
	cmd, err := s.pool.Exec(ctx, q, id, userID, status)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetReplicas updates the desired replica count for an app.
func (s *AppStore) SetReplicas(ctx context.Context, userID, id string, replicas int) error {
	const q = `UPDATE apps SET replicas = $3, updated_at = NOW() WHERE id = $1 AND user_id = $2`
	cmd, err := s.pool.Exec(ctx, q, id, userID, replicas)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes an app row. Cascading FK rules clean up deployments/containers.
func (s *AppStore) Delete(ctx context.Context, userID, id string) error {
	const q = `DELETE FROM apps WHERE id = $1 AND user_id = $2`
	cmd, err := s.pool.Exec(ctx, q, id, userID)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
