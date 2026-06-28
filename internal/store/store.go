// Package store wraps the Postgres connection pool (pgx) and runs database
// migrations. Per-entity repositories live in their own files (user_store.go,
// app_store.go, deploy_store.go) and share the single pool.
package store

import (
	"context"
	"fmt"
	"io/fs"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5" // pgx migrate driver
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/google/uuid"

	// Embedded SQL migrations. The whole migrations directory ships inside the
	// binary so `forgeos` needs no external migration files at runtime.
	migrationsFS "forgeos/migrations"
)

// migrationsEmbed is the embedded *.sql migration tree. Aliased so the source
// driver below reads from the package's own embed.FS instead of the filesystem.
var migrationsEmbed = migrationsFS.FS

// Store is the shared access point for the database. It owns the connection
// pool and exposes typed repositories.
type Store struct {
	Pool   *pgxpool.Pool
	Users  *UserStore
	Apps   *AppStore
	Deploy *DeployStore
	Builds *BuildStore
}

// New connects to Postgres, pings it, applies migrations, and returns a Store
// wired with all repositories. The caller is responsible for closing the pool.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	// Sensible pool defaults for a single-node control plane.
	cfg.MaxConns = 10
	cfg.MinConns = 1
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := runMigrations(databaseURL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	s := &Store{Pool: pool}
	s.Users = NewUserStore(pool)
	s.Apps = NewAppStore(pool)
	s.Deploy = NewDeployStore(pool)
	s.Builds = NewBuildStore(pool)
	return s, nil
}

// Close releases the connection pool.
func (s *Store) Close() {
	if s.Pool != nil {
		s.Pool.Close()
	}
}

// runMigrations embeds the migrations directory and applies all up migrations.
// It is idempotent: a fresh database and an existing one both end up current.
func runMigrations(databaseURL string) error {
	src, err := iofs.New(migrationsEmbed, ".")
	if err != nil {
		return fmt.Errorf("load embedded migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return fmt.Errorf("init migrate: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// NewUUID returns a new v4 UUID string. Centralized so stores/tests can mock it
// if needed; today it is a thin wrapper around google/uuid.
func NewUUID() string {
	return uuid.NewString()
}

// Compile-time guard: migrations must be embedded with mode read-only.
var _ fs.FS = migrationsEmbed
