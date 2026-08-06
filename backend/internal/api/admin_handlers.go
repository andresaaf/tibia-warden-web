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

// handleAdminPromote grants a user site-wide admin access. A banned user must
// be unbanned first (a banned admin is a contradictory state).
func (s *Server) handleAdminPromote(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	target, err := s.stores.Users.GetByID(r.Context(), targetID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load user")
		return
	}
	if target.Banned {
		writeError(w, http.StatusBadRequest, "unban the user before promoting")
		return
	}
	if err := s.stores.Users.SetAdmin(r.Context(), targetID, true); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to promote user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "promoted"})
}

// handleAdminDemote revokes a user's admin access. Admins cannot demote
// themselves, to avoid locking the last admin out of the panel.
func (s *Server) handleAdminDemote(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	if targetID == userID(r) {
		writeError(w, http.StatusBadRequest, "you cannot remove your own admin access")
		return
	}
	if err := s.stores.Users.SetAdmin(r.Context(), targetID, false); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to demote user")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "demoted"})
}

// handleAdminSetCharacterName lets an admin rename a user's Tibia character
// (e.g. to clear an offensive name). Mirrors the self-service validation.
func (s *Server) handleAdminSetCharacterName(w http.ResponseWriter, r *http.Request) {
	targetID, ok := parseID(w, r, "userID")
	if !ok {
		return
	}
	var body struct {
		CharacterName string `json:"characterName"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.CharacterName)
	if name == "" || len(name) > 60 {
		writeError(w, http.StatusBadRequest, "character name must be between 1 and 60 characters")
		return
	}
	user, err := s.stores.Users.SetCharacterName(r.Context(), targetID, name)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "user not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update character name")
		return
	}
	writeJSON(w, http.StatusOK, user)
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
