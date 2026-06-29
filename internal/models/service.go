package models

import "time"

// Service types
const (
	ServiceTypePostgres = "postgres"
	ServiceTypeRedis    = "redis"
)

// Service statuses
const (
	ServiceStatusCreating = "creating"
	ServiceStatusRunning  = "running"
	ServiceStatusFailed   = "failed"
)

// Service represents a managed database or caching service.
type Service struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Status      string    `json:"status"`
	InternalURL string    `json:"internal_url"`
	CreatedAt   time.Time `json:"created_at"`
}

// CreateServiceRequest is the body for POST /services.
type CreateServiceRequest struct {
	Name string `json:"name"`
	Type string `json:"type"` // "postgres" or "redis"
}
