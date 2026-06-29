package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// EnvStore handles CRUD for environment variables.
type EnvStore struct {
	pool *pgxpool.Pool
}

// NewEnvStore creates a new EnvStore.
func NewEnvStore(pool *pgxpool.Pool) *EnvStore {
	return &EnvStore{pool: pool}
}

// ListByApp returns all environment variables for a given app.
func (s *EnvStore) ListByApp(ctx context.Context, appID string) ([]*models.EnvVar, error) {
	const q = `
		SELECT id, app_id, key, value, is_secret, created_at, updated_at
		FROM env_vars
		WHERE app_id = $1
		ORDER BY key ASC`
	rows, err := s.pool.Query(ctx, q, appID)
	if err != nil {
		return nil, fmt.Errorf("list env vars: %w", err)
	}
	defer rows.Close()

	var vars []*models.EnvVar
	for rows.Next() {
		v := &models.EnvVar{}
		if err := rows.Scan(&v.ID, &v.AppID, &v.Key, &v.Value, &v.IsSecret, &v.CreatedAt, &v.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan env var: %w", err)
		}
		vars = append(vars, v)
	}
	return vars, rows.Err()
}

// ReplaceAll atomically replaces all environment variables for an app.
func (s *EnvStore) ReplaceAll(ctx context.Context, appID string, vars []models.EnvVarInput) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	// 1. Delete all existing env vars for this app
	if _, err := tx.Exec(ctx, `DELETE FROM env_vars WHERE app_id = $1`, appID); err != nil {
		return fmt.Errorf("delete old env vars: %w", err)
	}

	// 2. Insert new ones
	if len(vars) > 0 {
		// Use pgx Batch or just a loop since it's a small number usually
		batch := &pgx.Batch{}
		const q = `
			INSERT INTO env_vars (app_id, key, value, is_secret)
			VALUES ($1, $2, $3, $4)`
		for _, v := range vars {
			isSecret := true
			if v.IsSecret != nil {
				isSecret = *v.IsSecret
			}
			batch.Queue(q, appID, v.Key, v.Value, isSecret)
		}
		
		br := tx.SendBatch(ctx, batch)
		for i := 0; i < len(vars); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("insert env var %d: %w", i, mapPgErr(err))
			}
		}
		br.Close()
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	return nil
}
