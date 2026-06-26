package handlers

import (
	"encoding/json"
	"net/http"
)

// decodeJSON decodes the request body into v. It enforces a non-empty body and
// rejects unknown fields so typos surface early.
func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
