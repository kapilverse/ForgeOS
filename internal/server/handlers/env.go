package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"forgeos/internal/models"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// EnvHandler handles CRUD for app environment variables.
type EnvHandler struct {
	apps *store.AppStore
	env  *store.EnvStore
}

// NewEnvHandler wires the stores.
func NewEnvHandler(apps *store.AppStore, env *store.EnvStore) *EnvHandler {
	return &EnvHandler{apps: apps, env: env}
}

// List handles GET /apps/{id}/env
func (h *EnvHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	appID := chi.URLParam(r, "id")

	// Ensure user owns the app
	_, err := h.apps.GetByID(r.Context(), user.ID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	vars, err := h.env.ListByApp(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if vars == nil {
		vars = []*models.EnvVar{}
	}
	writeJSON(w, http.StatusOK, vars)
}

// ReplaceAll handles PUT /apps/{id}/env
func (h *EnvHandler) ReplaceAll(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	appID := chi.URLParam(r, "id")

	// Ensure user owns the app
	_, err := h.apps.GetByID(r.Context(), user.ID, appID)
	if err != nil {
		mapStoreErr(w, err)
		return
	}

	var req models.PutEnvRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.env.ReplaceAll(r.Context(), appID, req.Variables); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Fetch the updated list to return it
	vars, err := h.env.ListByApp(r.Context(), appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if vars == nil {
		vars = []*models.EnvVar{}
	}
	writeJSON(w, http.StatusOK, vars)
}
