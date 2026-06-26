package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"forgeos/internal/models"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// AppHandler handles app CRUD and lifecycle endpoints.
type AppHandler struct {
	apps     *store.AppStore
	deploy   *store.DeployStore
	deployer AppLifecycle // start/stop/delete side-effects (may be nil)
}

// AppLifecycle is the subset of deployer operations the app handler needs.
// Defined as an interface so handlers stay decoupled from the deployer package
// (and stay testable with a fake). Methods take a context and the loaded app.
type AppLifecycle interface {
	StopAppWithCtx(ctx context.Context, app *models.App) error
	DeleteAppWithCtx(ctx context.Context, app *models.App) error
}

// NewAppHandler wires the stores and the lifecycle collaborator.
func NewAppHandler(apps *store.AppStore, deploy *store.DeployStore, deployer AppLifecycle) *AppHandler {
	return &AppHandler{apps: apps, deploy: deploy, deployer: deployer}
}

// List handles GET /apps.
func (h *AppHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	apps, err := h.apps.ListByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if apps == nil {
		apps = []*models.App{}
	}
	writeJSON(w, http.StatusOK, apps)
}

// Create handles POST /apps.
func (h *AppHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	var req models.CreateAppRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	slug := slugify(req.Name)
	if !isValidSlug(slug) {
		writeError(w, http.StatusBadRequest, "name must be alphanumeric with dashes")
		return
	}

	app := &models.App{
		ID:          store.NewUUID(),
		UserID:      user.ID,
		Name:        req.Name,
		Slug:        slug,
		Description: strings.TrimSpace(req.Description),
		DockerImage: strings.TrimSpace(req.DockerImage),
		Status:      models.AppStatusCreated,
		Replicas:    ptrOr(req.Replicas, 1),
		CPULimit:    ptrOr(req.CPULimit, 512),
		MemoryLimit: ptrOr(req.MemoryLimit, 512),
		Port:        ptrOr(req.Port, 8080),
		HealthCheck: orDefault(strings.TrimSpace(req.HealthCheck), "/"),
	}
	if app.Replicas < 1 {
		app.Replicas = 1
	}
	if err := h.apps.Create(r.Context(), app); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "app name or slug already exists")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, app)
}

// Get handles GET /apps/{id}.
func (h *AppHandler) Get(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, ok := h.loadApp(w, r, user.ID)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// Update handles PATCH /apps/{id}.
func (h *AppHandler) Update(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, ok := h.loadApp(w, r, user.ID)
	if !ok {
		return
	}
	var req models.UpdateAppRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Description != nil {
		app.Description = *req.Description
	}
	if req.CPULimit != nil {
		app.CPULimit = *req.CPULimit
	}
	if req.MemoryLimit != nil {
		app.MemoryLimit = *req.MemoryLimit
	}
	if req.Port != nil {
		app.Port = *req.Port
	}
	if req.HealthCheck != nil {
		app.HealthCheck = *req.HealthCheck
	}
	if err := h.apps.Update(r.Context(), app); err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, app)
}

// Delete handles DELETE /apps/{id} — stops containers then removes the app.
func (h *AppHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, ok := h.loadApp(w, r, user.ID)
	if !ok {
		return
	}
	if h.deployer != nil {
		if err := h.deployer.DeleteAppWithCtx(r.Context(), app); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Stop handles POST /apps/{id}/stop.
func (h *AppHandler) Stop(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, ok := h.loadApp(w, r, user.ID)
	if !ok {
		return
	}
	if h.deployer != nil {
		if err := h.deployer.StopAppWithCtx(r.Context(), app); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

// Scale handles PATCH /apps/{id}/scale.
func (h *AppHandler) Scale(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, ok := h.loadApp(w, r, user.ID)
	if !ok {
		return
	}
	var req models.ScaleRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Replicas < 1 || req.Replicas > 10 {
		writeError(w, http.StatusBadRequest, "replicas must be between 1 and 10")
		return
	}
	if err := h.apps.SetReplicas(r.Context(), user.ID, app.ID, req.Replicas); err != nil {
		mapStoreErr(w, err)
		return
	}
	app.Replicas = req.Replicas
	writeJSON(w, http.StatusOK, app)
}

// loadApp fetches an app owned by the current user and writes a 404 on miss.
// Returns ok=false if the response was already written.
func (h *AppHandler) loadApp(w http.ResponseWriter, r *http.Request, userID string) (*models.App, bool) {
	id := chi.URLParam(r, "id")
	app, err := h.apps.GetByID(r.Context(), userID, id)
	if err != nil {
		mapStoreErr(w, err)
		return nil, false
	}
	return app, true
}

// --- slug helpers ---------------------------------------------------------

var slugRE = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

// slugify turns a display name into a URL-safe, lowercased slug.
func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// collapse non-alphanumerics and trim leading/trailing dashes
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash && b.Len() > 0 {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func isValidSlug(s string) bool { return s != "" && slugRE.MatchString(s) }

// ptrOr returns *v when non-nil, else fallback.
func ptrOr[T any](v *T, fallback T) T {
	if v != nil {
		return *v
	}
	return fallback
}

// orDefault returns s when non-empty, else fallback.
func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

// Unused but kept for future endpoints that parse ints from query strings.
var _ = strconv.Itoa
