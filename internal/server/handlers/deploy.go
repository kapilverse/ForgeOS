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
	BuildAndDeploy(ctx context.Context, app *models.App, repoURL, branch string) (*models.Deployment, error)
}

// DeployHandler handles deploy + deployment-history endpoints.
type DeployHandler struct {
	apps   *store.AppStore
	deploy *store.DeployStore
	builds *store.BuildStore
	engine DeployEngine
}

// NewDeployHandler wires the stores and the deployment engine.
func NewDeployHandler(apps *store.AppStore, deploy *store.DeployStore, builds *store.BuildStore, engine DeployEngine) *DeployHandler {
	return &DeployHandler{apps: apps, deploy: deploy, builds: builds, engine: engine}
}

// Deploy handles POST /apps/{id}/deploy — rolls out an image or builds from git.
func (h *DeployHandler) Deploy(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	app, err := h.apps.GetByID(r.Context(), user.ID, chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	var req models.DeployRequest
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
	}

	repoURL := strings.TrimSpace(req.RepoURL)
	if repoURL == "" {
		repoURL = strings.TrimSpace(app.RepoURL)
	}
	
	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = strings.TrimSpace(app.Branch)
	}

	imageRef := strings.TrimSpace(req.Image)
	if imageRef == "" {
		imageRef = strings.TrimSpace(app.DockerImage)
	}

	if repoURL != "" {
		// Build pipeline path
		deployment, err := h.engine.BuildAndDeploy(r.Context(), app, repoURL, branch)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusAccepted, deployment)
		return
	}

	if imageRef == "" {
		writeError(w, http.StatusBadRequest, "repo_url or image is required (set in body or app settings)")
		return
	}

	// Image deploy path
	deployment, err := h.engine.DeployImage(r.Context(), app, imageRef)
	if err != nil && !errors.Is(err, context.Canceled) {
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

// GetBuildLog handles GET /deployments/{id}/build.
func (h *DeployHandler) GetBuildLog(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	deploymentID := chi.URLParam(r, "id")

	// Verify the user owns this deployment by loading the app via the deployment.
	// Since we don't have a GetDeployment route that returns the app ID in this handler currently, 
	// we use a shortcut: load deployment to get app_id, verify app belongs to user.
	// We'll rely on the DB store to do the joins or we can do it directly.
	// A quick way is to trust the store if it had a method. For now, let's just 
	// get the build by deployment ID. It's a slight authorization gap if we don't check the app owner, 
	// but let's assume BuildStore's GetByDeploymentID is enough for now or we check it.
	
	// Better approach: verify ownership. 
	deployment, err := h.deploy.GetDeployment(r.Context(), deploymentID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	
	_, err = h.apps.GetByID(r.Context(), user.ID, deployment.AppID)
	if err != nil {
		mapStoreErr(w, err) // This handles 404 nicely.
		return
	}

	build, err := h.builds.GetByDeployment(r.Context(), deploymentID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	writeJSON(w, http.StatusOK, build)
}
