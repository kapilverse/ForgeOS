// Package config holds all runtime configuration for the ForgeOS control plane.
// Values are read from environment variables (optionally seeded from a local
// .env file via godotenv) and validated once at startup.
package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

// Config is the resolved configuration for a ForgeOS server instance.
type Config struct {
	DatabaseURL    string
	JWTSecret      []byte
	ListenAddr     string
	DockerNetwork  string
	ForgeOSDomain  string
	AllowedOrigins []string
}

// Load reads optional .env file then resolves all required env vars.
// Missing required values cause a fatal error so misconfiguration fails fast.
func Load() (*Config, error) {
	// .env is optional; ignore "not found" — explicit env vars still win.
	_ = godotenv.Load()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 16 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 16 characters")
	}

	listenAddr := os.Getenv("LISTEN_ADDR")
	if listenAddr == "" {
		listenAddr = ":8081"
	}

	network := os.Getenv("DOCKER_NETWORK")
	if network == "" {
		network = "forgeos"
	}

	domain := os.Getenv("FORGEOS_DOMAIN")
	if domain == "" {
		domain = "forgeos.local"
	}

	origins := []string{"*"}
	if raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS")); raw != "" {
		origins = strings.Split(raw, ",")
		for i := range origins {
			origins[i] = strings.TrimSpace(origins[i])
		}
	}

	return &Config{
		DatabaseURL:    dbURL,
		JWTSecret:      []byte(jwtSecret),
		ListenAddr:     listenAddr,
		DockerNetwork:  network,
		ForgeOSDomain:  domain,
		AllowedOrigins: origins,
	}, nil
}
