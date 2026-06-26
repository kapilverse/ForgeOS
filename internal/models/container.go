package models

import "time"

// Container lifecycle status.
const (
	ContainerStatusStarting = "starting"
	ContainerStatusRunning  = "running"
	ContainerStatusStopped  = "stopped"
	ContainerStatusCrashed  = "crashed"
)

// Container tracks a Docker container that ForgeOS manages for an app.
type Container struct {
	ID            string     `json:"id"`
	AppID         string     `json:"app_id"`
	DeploymentID  string     `json:"deployment_id"`
	ContainerID   string     `json:"container_id"`
	Name          string     `json:"name"`
	Status        string     `json:"status"`
	PortMapping   int        `json:"port_mapping,omitempty"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	StoppedAt     *time.Time `json:"stopped_at,omitempty"`
	RestartCount  int        `json:"restart_count"`
	CreatedAt     time.Time  `json:"created_at"`
}
