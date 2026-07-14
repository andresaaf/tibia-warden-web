package api

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

// writeJSON serializes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// writeError sends a JSON error body with the given status code.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// decodeJSON reads and validates a JSON request body into v.
func decodeJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}
