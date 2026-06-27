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
//
// Two deploy modes:
//   - Image mode: supply Image to deploy a pre-built Docker image directly.
//   - Build mode: supply RepoURL to clone a git repo, auto-generate a
//     Dockerfile, build the image, then deploy. Branch is optional
//     (defaults to the app's configured branch or "main").
//
// If neither field is supplied, the app's stored DockerImage / RepoURL is used.
type DeployRequest struct {
	Image   string `json:"image"`
	RepoURL string `json:"repo_url"`
	Branch  string `json:"branch"`
}
