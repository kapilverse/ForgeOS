package handlers

import (
	"encoding/json"
	"net/http"

	"forgeos/internal/store"
)

// writeJSON writes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeError writes a single-field JSON error with the given status.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// mapStoreErr translates store sentinel errors into appropriate HTTP statuses.
func mapStoreErr(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case err == store.ErrNotFound:
		writeError(w, http.StatusNotFound, "not found")
	case err == store.ErrConflict:
		writeError(w, http.StatusConflict, "resource already exists")
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
