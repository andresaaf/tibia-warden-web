package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
)

// handleAdminListUsers returns users for the admin panel, optionally filtered
// by a search string on Discord username or character name.
func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	search := strings.TrimSpace(r.URL.Query().Get("search"))
	users, err := s.stores.Users.ListForAdmin(r.Context(), search)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load users")
		return
	}
	if users == nil {
		users = []models.User{}
	}
	writeJSON(w, http.StatusOK, users)
}

// handleAdminBanUser bans a user from the website and clears their sessions so
// the ban takes effect immediately. Admins and the caller cannot be banned.
func (s *Server) handleAdminBanUser(w http.ResponseWriter, r *http.Request) {
	targetID, ok := s.adminBanTarget(w, r)
	if !ok {
		return
	}
	if err := s.stores.Users.SetBanned(r.Context(), targetID, true); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to ban user")
		return
	}
	if err := s.stores.Sessions.DeleteByUser(r.Context(), targetID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear sessions")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "banned"})
}

// handleAdminUnbanUser lifts a user's ban.
func (s *Server) handleAdminUnbanUser(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	if err := s.stores.Users.SetBanned(r.Context(), targetID, false); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to unban user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbanned"})
}

// adminBanTarget parses the target user ID and enforces the ban guardrails:
// an admin cannot ban themselves or another admin.
func (s *Server) adminBanTarget(w http.ResponseWriter, r *http.Request) (int64, bool) {
	targetID, err := strconv.ParseInt(chiURLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return 0, false
	}
	if targetID == userID(r) {
		writeError(w, http.StatusBadRequest, "you cannot ban yourself")
		return 0, false
	}
	target, err := s.stores.Users.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return 0, false
		}
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return 0, false
	}
	if target.IsAdmin {
		writeError(w, http.StatusBadRequest, "cannot ban an admin")
		return 0, false
	}
	return targetID, true
}
