package server

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"forgeos/internal/config"
	"forgeos/internal/container"
	"forgeos/internal/deployer"
	"forgeos/internal/router"
	"forgeos/internal/server/handlers"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// Deps bundles the shared dependencies each handler needs.
type Deps struct {
	Config *config.Config
	Store  *store.Store
	CM     *container.Manager
	Router *router.Traefik
	Deploy *deployer.Deployer
}

// New creates a chi router with all ForgeOS API routes wired.
func New(deps Deps) *chi.Mux {
	r := chi.NewRouter()

	// --- Global middleware ------------------------------------------------
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(middleware.Logger{Logger: log.Default()}.Middleware)
	r.Use(middleware.CORS(deps.Config.AllowedOrigins))

	auth := middleware.NewAuthenticator(deps.Config.JWTSecret, deps.Store.Users)

	// --- Health / system (no auth required) ------------------------------
	r.Get("/api/v1/system/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// --- Public auth endpoints -------------------------------------------
	authH := handlers.NewAuthHandler(deps.Store.Users, auth)
	r.Post("/api/v1/auth/register", authH.Register)
	r.Post("/api/v1/auth/login", authH.Login)

	// --- Protected routes -------------------------------------------------
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(auth.RequireAuth)

		r.Get("/auth/me", authH.Me)
		r.Post("/auth/regenerate-key", authH.RegenerateKey)

		// App lifecycle interface adapts the deployer for handler use.
		var lifecycle handlers.AppLifecycle // nil is safe: CRUD works without it
		if deps.Deploy != nil {
			lifecycle = deps.Deploy
		}

		appH := handlers.NewAppHandler(deps.Store.Apps, deps.Store.Deploy, lifecycle)
		r.Route("/apps", func(r chi.Router) {
			r.Get("/", appH.List)
			r.Post("/", appH.Create)

			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", appH.Get)
				r.Patch("/", appH.Update)
				r.Delete("/", appH.Delete)
				r.Post("/stop", appH.Stop)
				r.Patch("/scale", appH.Scale)

				// Deployment sub-routes.
				var engine handlers.DeployEngine // nil is safe: listing works
				if deps.Deploy != nil {
					engine = deps.Deploy
				}
				deployH := handlers.NewDeployHandler(deps.Store.Apps, deps.Store.Deploy, deps.Store.Builds, engine)
				r.Post("/deploy", deployH.Deploy)
				r.Get("/deployments", deployH.ListDeployments)
				r.Post("/deployments/{dep_id}/rollback", deployH.Rollback)
			})
		})

		// Global deployments routes
		deployHGlobal := handlers.NewDeployHandler(deps.Store.Apps, deps.Store.Deploy, deps.Store.Builds, nil)
		r.Route("/deployments", func(r chi.Router) {
			r.Get("/{id}/build", deployHGlobal.GetBuildLog)
		})
	})

	return r
}

// ListenAndServeGraceful starts the HTTP server and returns the *http.Server so
// the caller can call Shutdown on it when an OS signal arrives.
func ListenAndServeGraceful(addr string, handler http.Handler) (*http.Server, error) {
	srv := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	return srv, nil
}

// Shutdown triggers a graceful shutdown with a timeout.
func Shutdown(srv *http.Server, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}
