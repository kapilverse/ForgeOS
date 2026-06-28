package models

import "time"

// App lifecycle status.
const (
	AppStatusCreated   = "created"
	AppStatusDeploying = "deploying"
	AppStatusLive      = "live"
	AppStatusStopped   = "stopped"
	AppStatusError     = "error"
)

// App represents a user-deployable application.
type App struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	DockerImage string    `json:"docker_image,omitempty"`
	RepoURL     string    `json:"repo_url,omitempty"`
	Branch      string    `json:"branch,omitempty"`
	Status      string    `json:"status"`
	Replicas    int       `json:"replicas"`
	CPULimit    int       `json:"cpu_limit"`
	MemoryLimit int       `json:"memory_limit"`
	Port        int       `json:"port"`
	HealthCheck string    `json:"health_check"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// CreateAppRequest is the body for POST /apps.
type CreateAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	DockerImage string `json:"docker_image"`
	RepoURL     string `json:"repo_url"`
	Branch      string `json:"branch"`
	Replicas    *int   `json:"replicas"`
	CPULimit    *int   `json:"cpu_limit"`
	MemoryLimit *int   `json:"memory_limit"`
	Port        *int   `json:"port"`
	HealthCheck string `json:"health_check"`
}

// UpdateAppRequest is the body for PATCH /apps/:id. All fields optional.
type UpdateAppRequest struct {
	Description *string `json:"description"`
	CPULimit    *int    `json:"cpu_limit"`
	MemoryLimit *int    `json:"memory_limit"`
	Port        *int    `json:"port"`
	HealthCheck *string `json:"health_check"`
}

// ScaleRequest is the body for PATCH /apps/:id/scale.
type ScaleRequest struct {
	Replicas int `json:"replicas"`
}
