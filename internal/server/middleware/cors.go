package middleware

import (
	"net/http"
)

// CORS returns middleware that sets permissive cross-origin headers. Allowed
// origins come from config; the special "*" allows all origins (with
// ACAO echoed back to the request's Origin).
func CORS(allowed []string) func(http.Handler) http.Handler {
	allowAll := false
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, o := range allowed {
		if o == "*" {
			allowAll = true
		}
		allowedSet[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			switch {
			case allowAll && origin != "":
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Vary", "Origin")
			case origin != "":
				if _, ok := allowedSet[origin]; ok {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Api-Key")
			w.Header().Set("Access-Control-Allow-Credentials", "true")

			// Preflight short-circuit.
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
