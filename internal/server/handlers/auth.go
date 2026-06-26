package handlers

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/crypto/bcrypt"

	"forgeos/internal/models"
	"forgeos/internal/server/middleware"
	"forgeos/internal/store"
)

// AuthHandler exposes register/login/me/regenerate-key endpoints.
type AuthHandler struct {
	users *store.UserStore
	auth  *middleware.Authenticator
}

// NewAuthHandler wires the user store and authenticator.
func NewAuthHandler(users *store.UserStore, auth *middleware.Authenticator) *AuthHandler {
	return &AuthHandler{users: users, auth: auth}
}

// Register handles POST /auth/register.
func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req models.RegisterRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Name = strings.TrimSpace(req.Name)
	if req.Email == "" || req.Password == "" || req.Name == "" {
		writeError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}
	if len(req.Password) < 8 {
		writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to hash password")
		return
	}

	user := &models.User{
		ID:           store.NewUUID(),
		Email:        req.Email,
		PasswordHash: string(hash),
		Name:         req.Name,
		APIKey:       newAPIKey(),
	}
	if err := h.users.Create(r.Context(), user); err != nil {
		if errors.Is(err, store.ErrConflict) {
			writeError(w, http.StatusConflict, "email already registered")
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	token, err := h.auth.IssueToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusCreated, models.AuthResponse{Token: token, User: *user})
}

// Login handles POST /auth/login.
func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req models.LoginRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))

	user, err := h.users.GetByEmail(r.Context(), req.Email)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	token, err := h.auth.IssueToken(user.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to issue token")
		return
	}
	writeJSON(w, http.StatusOK, models.AuthResponse{Token: token, User: *user})
}

// Me handles GET /auth/me — returns the authenticated user.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	writeJSON(w, http.StatusOK, user)
}

// RegenerateKey handles POST /auth/regenerate-key — rotates the API key.
func (h *AuthHandler) RegenerateKey(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	newKey := newAPIKey()
	if err := h.users.UpdateAPIKey(r.Context(), user.ID, newKey); err != nil {
		mapStoreErr(w, err)
		return
	}
	user.APIKey = newKey
	writeJSON(w, http.StatusOK, user)
}

// newAPIKey returns a cryptographically random 32-byte hex API key.
func newAPIKey() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// rand.Read failing is catastrophic; there is no safe fallback.
		panic("crypto/rand failed: " + err.Error())
	}
	return "fo_" + hex.EncodeToString(b)
}
