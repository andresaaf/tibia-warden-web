package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/baz/tibia-warden-web/backend/internal/models"
	"github.com/baz/tibia-warden-web/backend/internal/store"
)

// Event type names broadcast over the group WebSocket.
const (
	eventAnnouncementCreated = "announcement.created"
	eventAnnouncementUpdated = "announcement.updated"
)

// handleListAnnouncements returns recent announcements for a group.
func (s *Server) handleListAnnouncements(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if _, err := s.requireMembership(r, groupID); err != nil {
		writeMembershipError(w, err)
		return
	}
	announcements, err := s.stores.Announcements.ListByGroup(r.Context(), groupID, 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load announcements")
		return
	}
	if announcements == nil {
		announcements = []models.Announcement{}
	}
	writeJSON(w, http.StatusOK, announcements)
}

// handleCreateAnnouncement posts a new Echo Warden reveal to a group.
func (s *Server) handleCreateAnnouncement(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if _, err := s.requireMembership(r, groupID); err != nil {
		writeMembershipError(w, err)
		return
	}

	var body struct {
		CreatureID int64  `json:"creatureId"`
		Location   string `json:"location"`
		Note       string `json:"note"`
		GoldCost   int    `json:"goldCost"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.CreatureID <= 0 {
		writeError(w, http.StatusBadRequest, "a creature is required")
		return
	}
	exists, err := s.stores.Creatures.Exists(r.Context(), body.CreatureID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to verify creature")
		return
	}
	if !exists {
		writeError(w, http.StatusBadRequest, "unknown creature")
		return
	}
	if body.GoldCost < 0 {
		body.GoldCost = 0
	}

	announcement, err := s.stores.Announcements.Create(
		r.Context(), groupID, body.CreatureID, userID(r),
		strings.TrimSpace(body.Location), strings.TrimSpace(body.Note), body.GoldCost)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create announcement")
		return
	}

	s.hub.Broadcast(groupID, eventAnnouncementCreated, announcement)
	writeJSON(w, http.StatusCreated, announcement)
}

// handleSetResponse records the current user's coming/ready state.
func (s *Server) handleSetResponse(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Status != models.ResponseComing && body.Status != models.ResponseReady {
		writeError(w, http.StatusBadRequest, "status must be 'coming' or 'ready'")
		return
	}

	groupID, err := s.authorizeAnnouncement(r, announcementID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if err := s.stores.Announcements.SetResponse(r.Context(), announcementID, userID(r), body.Status); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save response")
		return
	}
	s.broadcastAnnouncement(r, groupID, announcementID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleClearResponse removes the current user's response.
func (s *Server) handleClearResponse(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	groupID, err := s.authorizeAnnouncement(r, announcementID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if err := s.stores.Announcements.ClearResponse(r.Context(), announcementID, userID(r)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear response")
		return
	}
	s.broadcastAnnouncement(r, groupID, announcementID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMarkAnnouncementKilled marks the reveal as killed (author only).
func (s *Server) handleMarkAnnouncementKilled(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	groupID, err := s.authorizeAnnouncement(r, announcementID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if err := s.stores.Announcements.MarkKilled(r.Context(), announcementID, userID(r)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusForbidden, "only the author can mark this as killed, and only once")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to mark as killed")
		return
	}
	s.broadcastAnnouncement(r, groupID, announcementID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "killed"})
}

// handleClaimAnnouncement records that the user obtained the benefit and ticks
// the creature on their warden list.
func (s *Server) handleClaimAnnouncement(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	groupID, err := s.authorizeAnnouncement(r, announcementID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if err := s.stores.Announcements.Claim(r.Context(), announcementID, userID(r)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "announcement must be marked killed before claiming")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to claim")
		return
	}
	s.broadcastAnnouncement(r, groupID, announcementID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "claimed"})
}

// authorizeAnnouncement resolves the group of an announcement and verifies the
// current user is a member of it. Returns the group ID on success.
func (s *Server) authorizeAnnouncement(r *http.Request, announcementID int64) (int64, error) {
	groupID, err := s.stores.Announcements.GroupID(r.Context(), announcementID)
	if err != nil {
		return 0, err
	}
	if _, err := s.stores.Groups.Role(r.Context(), groupID, userID(r)); err != nil {
		return 0, err
	}
	return groupID, nil
}

// broadcastAnnouncement reloads an announcement and pushes it to the group room.
func (s *Server) broadcastAnnouncement(r *http.Request, groupID, announcementID int64) {
	announcement, err := s.stores.Announcements.GetByID(r.Context(), announcementID)
	if err != nil {
		return
	}
	s.hub.Broadcast(groupID, eventAnnouncementUpdated, announcement)
}
