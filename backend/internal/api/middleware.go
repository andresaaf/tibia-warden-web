package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/andresaaf/tibia-warden-web/backend/internal/session"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
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
		userID, banned, err := s.stores.Sessions.AuthByToken(r.Context(), token)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusUnauthorized, "invalid or expired session")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to verify session")
			return
		}
		if banned {
			writeError(w, http.StatusForbidden, "account banned")
			return
		}
		ctx := context.WithValue(r.Context(), userIDKey, userID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin gates admin-only routes. It must run after requireAuth, which
// populates the user ID in the request context.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, err := s.stores.Users.GetByID(r.Context(), userID(r))
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to verify admin access")
			return
		}
		if !user.IsAdmin {
			writeError(w, http.StatusForbidden, "admin only")
			return
		}
		next.ServeHTTP(w, r.WithContext(r.Context()))
	})
}

// userID extracts the authenticated user ID from the request context.
func userID(r *http.Request) int64 {
	if v, ok := r.Context().Value(userIDKey).(int64); ok {
		return v
	}
	return 0
}
