package models

import "time"

// Deployment lifecycle status.
const (
	DeploymentStatusPending    = "pending"
	DeploymentStatusDeploying  = "deploying"
	DeploymentStatusLive       = "live"
	DeploymentStatusFailed     = "failed"
	DeploymentStatusRolledBack = "rolled_back"
)

// Deployment records a single rollout of an app image.
type Deployment struct {
	ID           string     `json:"id"`
	AppID        string     `json:"app_id"`
	Version      int        `json:"version"`
	ImageTag     string     `json:"image_tag"`
	Status       string     `json:"status"`
	ErrorMessage string     `json:"error_message,omitempty"`
	IsCurrent    bool       `json:"is_current"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}

// DeployRequest is the body for POST /apps/:id/deploy.
// Either Image (deploy a pre-built Docker image) is required for this
// increment. Repo-based builds arrive in a later sprint.
type DeployRequest struct {
	Image string `json:"image"`
}
