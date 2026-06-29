package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// ServiceStore handles CRUD for managed services (Postgres, Redis).
type ServiceStore struct {
	pool *pgxpool.Pool
}

// NewServiceStore creates a new ServiceStore.
func NewServiceStore(pool *pgxpool.Pool) *ServiceStore {
	return &ServiceStore{pool: pool}
}

// Create inserts a new service record.
func (s *ServiceStore) Create(ctx context.Context, svc *models.Service) error {
	const q = `
		INSERT INTO services (id, user_id, name, type, status, internal_url)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING created_at`
	return mapPgErr(s.pool.QueryRow(ctx, q,
		svc.ID, svc.UserID, svc.Name, svc.Type, svc.Status, svc.InternalURL,
	).Scan(&svc.CreatedAt))
}

// ListByUser returns all services owned by a user.
func (s *ServiceStore) ListByUser(ctx context.Context, userID string) ([]*models.Service, error) {
	const q = `
		SELECT id, user_id, name, type, status, internal_url, created_at
		FROM services
		WHERE user_id = $1
		ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("list services: %w", err)
	}
	defer rows.Close()

	var services []*models.Service
	for rows.Next() {
		svc := &models.Service{}
		if err := rows.Scan(
			&svc.ID, &svc.UserID, &svc.Name, &svc.Type, &svc.Status, &svc.InternalURL, &svc.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan service: %w", err)
		}
		services = append(services, svc)
	}
	return services, rows.Err()
}

// GetByID loads a single service.
func (s *ServiceStore) GetByID(ctx context.Context, userID, id string) (*models.Service, error) {
	const q = `
		SELECT id, user_id, name, type, status, internal_url, created_at
		FROM services
		WHERE user_id = $1 AND id = $2`
	svc := &models.Service{}
	err := s.pool.QueryRow(ctx, q, userID, id).Scan(
		&svc.ID, &svc.UserID, &svc.Name, &svc.Type, &svc.Status, &svc.InternalURL, &svc.CreatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return svc, nil
}

// Delete removes a service.
func (s *ServiceStore) Delete(ctx context.Context, userID, id string) error {
	cmd, err := s.pool.Exec(ctx, `DELETE FROM services WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// UpdateStatus changes the status of a service.
func (s *ServiceStore) UpdateStatus(ctx context.Context, id, status string) error {
	_, err := s.pool.Exec(ctx, `UPDATE services SET status = $2 WHERE id = $1`, id, status)
	return mapPgErr(err)
}

// UpdateInternalURL changes the internal URL of a service.
func (s *ServiceStore) UpdateInternalURL(ctx context.Context, id, url string) error {
	_, err := s.pool.Exec(ctx, `UPDATE services SET internal_url = $2 WHERE id = $1`, id, url)
	return mapPgErr(err)
}
