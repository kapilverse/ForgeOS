package models

import "time"

// EnvVar represents a single environment variable injected into an app's containers.
type EnvVar struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	Key       string    `json:"key"`
	Value     string    `json:"value"`
	IsSecret  bool      `json:"is_secret"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// EnvVarInput is used to set or update environment variables.
type EnvVarInput struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	IsSecret *bool  `json:"is_secret,omitempty"`
}

// PutEnvRequest is the body for PUT /apps/:id/env.
type PutEnvRequest struct {
	Variables []EnvVarInput `json:"variables"`
}
