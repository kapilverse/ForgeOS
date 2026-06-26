package models

import "time"

// User is a ForgeOS account. The API key authenticates the CLI; JWT tokens
// authenticate browser/dashboard sessions.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"` // never serialized
	Name         string    `json:"name"`
	APIKey       string    `json:"api_key"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RegisterRequest is the body for POST /auth/register.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

// LoginRequest is the body for POST /auth/login.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// AuthResponse is returned on register/login. The token is a signed JWT.
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
