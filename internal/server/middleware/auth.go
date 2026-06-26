package middleware

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"forgeos/internal/models"
	"forgeos/internal/store"
)

type ctxKey int

const (
	ctxUser ctxKey = iota
)

// claims is the JWT payload we sign for auth tokens.
type claims struct {
	UserID string `json:"uid"`
	jwt.RegisteredClaims
}

// Authenticator issues and verifies JWTs and loads the user on each request.
type Authenticator struct {
	secret []byte
	users  *store.UserStore
}

// NewAuthenticator wires the JWT secret and user store.
func NewAuthenticator(secret []byte, users *store.UserStore) *Authenticator {
	return &Authenticator{secret: secret, users: users}
}

// IssueToken signs a JWT for the given user id, valid for 7 days.
func (a *Authenticator) IssueToken(userID string) (string, error) {
	c := claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "forgeos",
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	return tok.SignedString(a.secret)
}

// RequireAuth is middleware that extracts and verifies a Bearer token from the
// Authorization header, loads the user, and stores it in the request context.
// Requests without a valid token get 401.
func (a *Authenticator) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userID, err := a.userIDFromRequest(r)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		user, err := a.users.GetByID(r.Context(), userID)
		if err != nil {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), ctxUser, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userIDFromRequest supports two schemes: Bearer JWT (Authorization header)
// and API-key (X-Api-Key header), the latter for CLI use.
func (a *Authenticator) userIDFromRequest(r *http.Request) (string, error) {
	// API key first.
	if key := r.Header.Get("X-Api-Key"); key != "" {
		user, err := a.users.GetByAPIKey(r.Context(), key)
		if err != nil {
			return "", err
		}
		return user.ID, nil
	}

	// Bearer JWT.
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", errors.New("missing authorization")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return "", errors.New("invalid authorization header")
	}
	tokenStr := strings.TrimPrefix(authHeader, prefix)

	tok, err := jwt.ParseWithClaims(tokenStr, &claims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return a.secret, nil
	})
	if err != nil || !tok.Valid {
		return "", errors.New("invalid token")
	}
	return tok.Claims.(*claims).UserID, nil
}

// UserFromContext extracts the authenticated user set by RequireAuth.
// Panics by design if called outside a protected route (programmer error).
func UserFromContext(ctx context.Context) *models.User {
	return ctx.Value(ctxUser).(*models.User)
}
