package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// chiURLParam returns a named URL parameter from the request route.
func chiURLParam(r *http.Request, key string) string {
	return chi.URLParam(r, key)
}
