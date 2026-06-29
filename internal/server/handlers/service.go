package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"forgeos/internal/container"
	"forgeos/internal/models"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// ServiceHandler handles managed databases/services.
type ServiceHandler struct {
	services   *store.ServiceStore
	containers *container.Manager
}

// NewServiceHandler returns a new ServiceHandler.
func NewServiceHandler(services *store.ServiceStore, containers *container.Manager) *ServiceHandler {
	return &ServiceHandler{services: services, containers: containers}
}

// Create handles POST /services
func (h *ServiceHandler) Create(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	
	var req models.CreateServiceRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" || req.Type == "" {
		writeError(w, http.StatusBadRequest, "name and type are required")
		return
	}
	
	if req.Type != models.ServiceTypePostgres && req.Type != models.ServiceTypeRedis {
		writeError(w, http.StatusBadRequest, "type must be postgres or redis")
		return
	}
	
	svc := &models.Service{
		ID:     store.NewUUID(),
		UserID: user.ID,
		Name:   req.Name,
		Type:   req.Type,
		Status: models.ServiceStatusCreating,
	}
	
	// Create database record
	if err := h.services.Create(r.Context(), svc); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	// Spin up the container
	go h.provision(user.ID, svc)

	writeJSON(w, http.StatusCreated, svc)
}

func (h *ServiceHandler) provision(userID string, svc *models.Service) {
	ctx := context.Background() // background task

	appID := fmt.Sprintf("service-%s", svc.ID) // used for labels
	
	cfg := container.ContainerConfig{
		AppID: appID,
		Port:  5432, // default for postgres, overridden below
	}
	
	password := store.NewUUID() // secure random pass
	
	if svc.Type == models.ServiceTypePostgres {
		cfg.Image = "postgres:15-alpine"
		cfg.Port = 5432
		cfg.EnvVars = map[string]string{
			"POSTGRES_USER":     "forgeos",
			"POSTGRES_PASSWORD": password,
			"POSTGRES_DB":       "forgeos",
		}
		// internal_url format: postgres://user:pass@host:port/db
		// We'll update the URL once we know the container name
	} else if svc.Type == models.ServiceTypeRedis {
		cfg.Image = "redis:7-alpine"
		cfg.Port = 6379
		// Redis defaults to no auth.
	}
	
	c, err := h.containers.Create(ctx, cfg)
	if err != nil {
		_ = h.services.UpdateStatus(ctx, svc.ID, models.ServiceStatusFailed)
		return
	}

	// Wait for running
	if err := h.containers.WaitForRunning(ctx, c.ID, 30*time.Second); err != nil {
		_ = h.services.UpdateStatus(ctx, svc.ID, models.ServiceStatusFailed)
		return
	}
	
	// Set the internal URL. The container's name acts as its DNS name in the Docker network.
	if svc.Type == models.ServiceTypePostgres {
		svc.InternalURL = fmt.Sprintf("postgres://forgeos:%s@%s:5432/forgeos", password, c.Name)
	} else if svc.Type == models.ServiceTypeRedis {
		svc.InternalURL = fmt.Sprintf("redis://%s:6379", c.Name)
	}

	// Manually update the URL in the database
	_ = h.services.UpdateInternalURL(ctx, svc.ID, svc.InternalURL)
	_ = h.services.UpdateStatus(ctx, svc.ID, models.ServiceStatusRunning)
}

// List handles GET /services
func (h *ServiceHandler) List(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	
	svcs, err := h.services.ListByUser(r.Context(), user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	if svcs == nil {
		svcs = []*models.Service{}
	}
	writeJSON(w, http.StatusOK, svcs)
}

// Delete handles DELETE /services/{id}
func (h *ServiceHandler) Delete(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	id := chi.URLParam(r, "id")
	
	// We need to fetch it to get the AppID (service-<id>) to delete containers
	svc, err := h.services.GetByID(r.Context(), user.ID, id)
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	
	appID := fmt.Sprintf("service-%s", svc.ID)
	h.containers.RemoveByApp(r.Context(), appID)
	
	if err := h.services.Delete(r.Context(), user.ID, id); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
