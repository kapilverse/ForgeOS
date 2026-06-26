package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forgeos/internal/models"
)

// ErrNotFound is returned when a single-row lookup matches nothing.
var ErrNotFound = errors.New("not found")

// ErrConflict is returned on a unique-constraint violation (e.g. duplicate email).
var ErrConflict = errors.New("conflict")

// UserStore handles user persistence.
type UserStore struct {
	pool *pgxpool.Pool
}

// NewUserStore returns a UserStore bound to the given pool.
func NewUserStore(pool *pgxpool.Pool) *UserStore {
	return &UserStore{pool: pool}
}

// Create inserts a new user. The caller is expected to pre-hash the password.
func (s *UserStore) Create(ctx context.Context, u *models.User) error {
	const q = `
		INSERT INTO users (id, email, password_hash, name, api_key)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING created_at, updated_at`
	err := s.pool.QueryRow(ctx, q,
		u.ID, u.Email, u.PasswordHash, u.Name, u.APIKey,
	).Scan(&u.CreatedAt, &u.UpdatedAt)
	return mapPgErr(err)
}

// GetByEmail loads a user by email address.
func (s *UserStore) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	u := &models.User{}
	const q = `
		SELECT id, email, password_hash, name, api_key, created_at, updated_at
		FROM users WHERE email = $1`
	err := s.pool.QueryRow(ctx, q, email).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.APIKey, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return u, nil
}

// GetByID loads a user by primary key.
func (s *UserStore) GetByID(ctx context.Context, id string) (*models.User, error) {
	u := &models.User{}
	const q = `
		SELECT id, email, password_hash, name, api_key, created_at, updated_at
		FROM users WHERE id = $1`
	err := s.pool.QueryRow(ctx, q, id).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.APIKey, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return u, nil
}

// GetByAPIKey loads a user by their CLI/API key.
func (s *UserStore) GetByAPIKey(ctx context.Context, apiKey string) (*models.User, error) {
	u := &models.User{}
	const q = `
		SELECT id, email, password_hash, name, api_key, created_at, updated_at
		FROM users WHERE api_key = $1`
	err := s.pool.QueryRow(ctx, q, apiKey).Scan(
		&u.ID, &u.Email, &u.PasswordHash, &u.Name, &u.APIKey, &u.CreatedAt, &u.UpdatedAt,
	)
	if err != nil {
		return nil, mapPgErr(err)
	}
	return u, nil
}

// UpdateAPIKey rotates a user's API key.
func (s *UserStore) UpdateAPIKey(ctx context.Context, id, apiKey string) error {
	const q = `UPDATE users SET api_key = $2, updated_at = NOW() WHERE id = $1`
	cmd, err := s.pool.Exec(ctx, q, id, apiKey)
	if err != nil {
		return mapPgErr(err)
	}
	if cmd.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// mapPgErr converts common pgx errors into the store's sentinel errors so
// handlers can map them to clean HTTP responses without leaking driver details.
func mapPgErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if isUniqueViolation(err) {
		return ErrConflict
	}
	return fmt.Errorf("database error: %w", err)
}
