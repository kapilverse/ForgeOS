package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"forgeos/internal/models"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// DeployEngine is the subset of deployer operations the deploy handler needs.
type DeployEngine interface {
	DeployImage(ctx context.Context, app *models.App, imageRef string) (*models.Deployment, error)
}

// DeployHandler handles deploy + deployment-history endpoints.
type DeployHandler struct {
	apps   *store.AppStore
	deploy *store.DeployStore
	engine DeployEngine
}

// NewDeployHandler wires the stores and the deployment engine.
func NewDeployHandler(apps *store.AppStore, deploy *store.DeployStore, engine DeployEngine) *DeployHandler {
	return &DeployHandler{apps: apps, deploy: deploy, engine: engine}
}

// Deploy handles POST /apps/{id}/deploy — rolls out an image.
func (h *DeployHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, err := h.apps.GetByID(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	var req models.DeployRequest
	// Body is optional if the app already has a docker_image configured.
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}
	imageRef := strings.TrimSpace(req.Image)
	if imageRef == "" {
		imageRef = strings.TrimSpace(app.DockerImage)
	}
	if imageRef == "" {
		writeError(w, http.StatusBadRequest, "image is required (set in body or app.docker_image)")
		return
	}

	deployment, err := h.engine.DeployImage(r.Context(), app, imageRef)
	if err != nil && !errors.Is(err, context.Canceled) {
		// The deployment row is still returned even on failure (status=failed),
		// so the client can inspect it. Surface a 202 to indicate the rollout
		// was processed but did not reach "live".
		writeJSON(w, http.StatusAccepted, deployment)
		return
	}
	writeJSON(w, http.StatusCreated, deployment)
}

// ListDeployments handles GET /apps/{id}/deployments.
func (h *DeployHandler) ListDeployments(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, err := h.apps.GetByID(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	deps, err := h.deploy.ListDeployments(r.Context(), app.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if deps == nil {
		deps = []*models.Deployment{}
	}
	writeJSON(w, http.StatusOK, deps)
}
