package models

import "time"

// Build lifecycle status.
const (
	BuildStatusPending = "pending"
	BuildStatusRunning = "running"
	BuildStatusSuccess = "success"
	BuildStatusFailed  = "failed"
)

// Build records the build phase of a deployment: the docker build log, how
// long it took, and its outcome. One build row per deployment that was built
// from source (image-mode deploys have no build row).
type Build struct {
	ID           string     `json:"id"`
	DeploymentID string     `json:"deployment_id"`
	Status       string     `json:"status"`
	Log          string     `json:"log,omitempty"`
	DurationMs   int        `json:"duration_ms,omitempty"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
