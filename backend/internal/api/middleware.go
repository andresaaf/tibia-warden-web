package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/baz/tibia-warden-web/backend/internal/session"
	"github.com/baz/tibia-warden-web/backend/internal/store"
)

type contextKey string

const userIDKey contextKey = "userID"

// requireAuth is middleware that resolves the session cookie to a user ID and
// stores it in the request context, rejecting unauthenticated requests.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token, ok := session.Token(r)
		if !ok {
			writeError(w, http.StatusUnauthorized, "authentication required")
			return
		}
		userID, err := s.stores.Sessions.UserIDByToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to verify session")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// userID extracts the authenticated user ID from the request context.
func userID(r *http.Request) int64 {
	if v, ok := r.Context().Value(userIDKey).(int64); ok {
		return v
	}
	return 0
}
