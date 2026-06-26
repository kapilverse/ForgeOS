// Package main is the ForgeOS API server entry point.
//
// Usage:
//
//	docker compose up -d          # postgres + traefik
//	cp .env.example .env          # configure DATABASE_URL, JWT_SECRET, etc.
//	go run ./cmd/server            # starts on :8081
//
// The server auto-runs database migrations on startup so no separate migrate
// step is needed.
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/api/types/network"

	"forgeos/internal/config"
	"forgeos/internal/container"
	"forgeos/internal/deployer"
	"forgeos/internal/router"
	"forgeos/internal/server"
	"forgeos/internal/store"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("forgeos: %v", err)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	log.Printf("config: listen=%s network=%s domain=%s", cfg.ListenAddr, cfg.DockerNetwork, cfg.ForgeOSDomain)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Database ----------------------------------------------------------
	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()
	log.Println("database: connected, migrations applied")

	// --- Docker ------------------------------------------------------------
	cm, err := container.New(ctx, cfg.DockerNetwork)
	if err != nil {
		return err
	}
	log.Println("docker: connected")

	// Ensure the shared network exists (Traefik + all app containers).
	if err := ensureNetwork(ctx, cm, cfg.DockerNetwork); err != nil {
		return err
	}

	// --- Traefik label generator ------------------------------------------
	rt := router.New(cfg.ForgeOSDomain, cfg.DockerNetwork)

	// --- Deployer ----------------------------------------------------------
	dep := deployer.New(cm, rt, db, log.Default())

	// --- HTTP server -------------------------------------------------------
	srvDeps := server.Deps{
		Config: cfg,
		Store:  db,
		CM:     cm,
		Router: rt,
		Deploy: dep,
	}
	handler := server.New(srvDeps)

	srv, err := server.ListenAndServeGraceful(cfg.ListenAddr, handler)
	if err != nil {
		return err
	}
	log.Printf("forgeos: API listening on %s", cfg.ListenAddr)

	// --- Graceful shutdown -------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down...")

	server.Shutdown(srv, 15*time.Second)
	log.Println("forgeos: stopped")
	return nil
}

// ensureNetwork creates the Docker bridge network if it does not already exist.
// This is idempotent; Traefik's docker-compose.yml should already create it,
// but we also create it here so the server works standalone without compose.
func ensureNetwork(ctx context.Context, cm *container.Manager, name string) error {
	cli := cm.Client()
	networks, err := cli.NetworkList(ctx, network.ListOptions{})
	if err != nil {
		return err
	}
	for _, n := range networks {
		if n.Name == name {
			log.Printf("docker: network %q already exists", name)
			return nil
		}
	}
	_, err = cli.NetworkCreate(ctx, name, network.CreateOptions{Driver: "bridge"})
	if err != nil {
		return err
	}
	log.Printf("docker: created network %q", name)
	return nil
}
