// Package router generates Traefik routing configuration for ForgeOS app
// containers. Traefik runs with its Docker provider and auto-discovers
// containers via labels, so routing is expressed entirely as Docker labels.
//
// In this sprint every replica for an app shares the same Traefik service name
// (forgeos-<slug>), so Traefik load-balances across all replicas automatically.
// Each replica gets its own router name to avoid collisions.
package router

import (
	"fmt"
	"strings"
)

// Traefik renders labels for a given base domain and Docker network.
type Traefik struct {
	domain  string
	network string
}

// New returns a Traefik label generator for the configured domain/network.
func New(domain, network string) *Traefik {
	return &Traefik{domain: domain, network: network}
}

// LabelConfig describes a single replica's routing.
type LabelConfig struct {
	Slug    string // app slug, e.g. "my-api"
	Port    int    // container port the app listens on
	Replica int    // 0-based replica index, used to derive a unique router name
}

// Labels builds the Docker labels for one replica. Traefik's Docker provider
// reads these and wires a Host()-based router + a load-balanced service.
func (t *Traefik) Labels(cfg LabelConfig) map[string]string {
	service := serviceName(cfg.Slug)            // shared across replicas -> LB
	router := routerName(cfg.Slug, cfg.Replica) // unique per replica
	host := t.host(cfg.Slug)

	labels := map[string]string{
		// Enable this container for Traefik (provider default is off).
		"traefik.enable": "true",
		// Only route over the shared ForgeOS network; ignore host networks.
		"traefik.docker.network": t.network,

		// Router: match the app's subdomain on the web (HTTP) entrypoint.
		fmt.Sprintf("traefik.http.routers.%s.rule", router):        fmt.Sprintf("Host(`%s`)", host),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints", router): "web",

		// All replicas of an app share a service so Traefik load-balances them.
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port", service): fmt.Sprintf("%d", cfg.Port),
		// Attach this replica's router to the shared service.
		fmt.Sprintf("traefik.http.routers.%s.service", router): service,
	}
	return labels
}

// host returns the full subdomain for an app slug, e.g. "my-api.forgeos.local".
func (t *Traefik) host(slug string) string {
	return fmt.Sprintf("%s.%s", slug, strings.TrimPrefix(t.domain, "."))
}

// serviceName is the Traefik service shared by all replicas of an app.
func serviceName(slug string) string { return "forgeos-" + slug }

// routerName is a unique Traefik router name per replica.
func routerName(slug string, replica int) string {
	return fmt.Sprintf("forgeos-%s-%d", slug, replica)
}
