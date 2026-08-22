package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
	"github.com/andresaaf/tibia-warden-web/backend/internal/ws"
)

// maxNoteLength caps announcement note length, staying under Discord's 1024-char
// embed field-value limit.
const maxNoteLength = 1000

// errForbidden signals an authorization failure whose HTTP response has already
// been written by the helper that returned it.
var errForbidden = errors.New("forbidden")

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
		MapX       *int   `json:"mapX"`
		MapY       *int   `json:"mapY"`
		MapZ       *int   `json:"mapZ"`
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
	mapX, mapY, mapZ := normalizeMapCoord(body.MapX, body.MapY, body.MapZ)

	announcement, err := s.stores.Announcements.Create(
		r.Context(), groupID, body.CreatureID, userID(r),
		strings.TrimSpace(body.Location), strings.TrimSpace(body.Note), body.GoldCost, nil, mapX, mapY, mapZ)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create announcement")
		return
	}

	s.hub.Broadcast(groupID, ws.EventAnnouncementCreated, announcement)
	s.bot.PostAnnouncement(r.Context(), announcement)
	writeJSON(w, http.StatusCreated, announcement)
}

// handleListFeed returns recent announcements across all the user's groups.
func (s *Server) handleListFeed(w http.ResponseWriter, r *http.Request) {
	announcements, err := s.stores.Announcements.ListForUser(r.Context(), userID(r), 50)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load feed")
		return
	}
	if announcements == nil {
		announcements = []models.Announcement{}
	}
	writeJSON(w, http.StatusOK, announcements)
}

// handleBroadcastAnnouncement posts an Echo Warden reveal to several groups at
// once (all of the user's groups when no groupIds are given).
func (s *Server) handleBroadcastAnnouncement(w http.ResponseWriter, r *http.Request) {
	var body struct {
		CreatureID int64   `json:"creatureId"`
		MapX       *int    `json:"mapX"`
		MapY       *int    `json:"mapY"`
		MapZ       *int    `json:"mapZ"`
		Note       string  `json:"note"`
		GoldCost   int     `json:"goldCost"`
		GroupIDs   []int64 `json:"groupIds"`
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

	uid := userID(r)
	var targets []int64
	if len(body.GroupIDs) > 0 {
		for _, gid := range body.GroupIDs {
			if _, err := s.stores.Groups.Role(r.Context(), gid, uid); err == nil {
				targets = append(targets, gid)
			}
		}
	} else {
		targets, err = s.stores.Groups.MemberGroupIDs(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load groups")
			return
		}
	}
	if len(targets) == 0 {
		writeError(w, http.StatusBadRequest, "you have no groups to announce to")
		return
	}

	note := strings.TrimSpace(body.Note)
	mapX, mapY, mapZ := normalizeMapCoord(body.MapX, body.MapY, body.MapZ)
	created := []models.Announcement{}

	// Link the fan-out so a kill on one cascades to the others.
	var broadcastID *string
	if len(targets) > 1 {
		if token, tErr := randomInviteCode(); tErr == nil {
			broadcastID = &token
		}
	}
	for _, gid := range targets {
		ann, err := s.stores.Announcements.Create(r.Context(), gid, body.CreatureID, uid, "", note, body.GoldCost, broadcastID, mapX, mapY, mapZ)
		if err != nil {
			continue
		}
		s.hub.Broadcast(gid, ws.EventAnnouncementCreated, ann)
		s.bot.PostAnnouncement(r.Context(), ann)
		created = append(created, *ann)
	}
	writeJSON(w, http.StatusCreated, created)
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

	if _, err := s.authorizeAnnouncement(r, announcementID); err != nil {
		writeMembershipError(w, err)
		return
	}
	affected, err := s.stores.Announcements.SetResponse(r.Context(), announcementID, userID(r), body.Status)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save response")
		return
	}
	for _, id := range affected {
		s.broadcastAnnouncement(r, id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleClearResponse removes the current user's response.
func (s *Server) handleClearResponse(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	if _, err := s.authorizeAnnouncement(r, announcementID); err != nil {
		writeMembershipError(w, err)
		return
	}
	affected, err := s.stores.Announcements.ClearResponse(r.Context(), announcementID, userID(r))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear response")
		return
	}
	for _, id := range affected {
		s.broadcastAnnouncement(r, id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// handleMarkAnnouncementKilled marks the reveal as killed (poster or group admin).
func (s *Server) handleMarkAnnouncementKilled(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	ann, err := s.authorizeAnnouncementManage(w, r, announcementID, "mark it killed")
	if err != nil {
		return
	}
	affected, err := s.stores.Announcements.MarkKilledWithSiblings(r.Context(), ann.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "this is already marked killed")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to mark as killed")
		return
	}
	// Notify each affected announcement (the primary plus any broadcast siblings).
	for _, id := range affected {
		a, err := s.stores.Announcements.GetByID(r.Context(), id)
		if err != nil {
			continue
		}
		s.hub.Broadcast(a.GroupID, ws.EventAnnouncementUpdated, a)
		s.bot.SyncAnnouncement(r.Context(), a)
		s.bot.OnAnnouncementKilled(r.Context(), id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "killed"})
}

// handleClaimAnnouncement records that the user obtained the benefit and ticks
// the creature on their warden list.
func (s *Server) handleClaimAnnouncement(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	if _, err := s.authorizeAnnouncement(r, announcementID); err != nil {
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
	s.broadcastAnnouncement(r, announcementID)
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

// authorizeAnnouncementManage loads an announcement and verifies the current user
// is allowed to manage it — i.e. is the author or a group owner/admin. On failure
// it writes the appropriate error response and returns a non-nil error, so callers
// can simply `return`. The action word is used in the 403 message ("only the
// person who announced it or a group admin can <action>").
func (s *Server) authorizeAnnouncementManage(w http.ResponseWriter, r *http.Request, announcementID int64, action string) (*models.Announcement, error) {
	uid := userID(r)
	ann, err := s.stores.Announcements.GetByID(r.Context(), announcementID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "announcement not found")
			return nil, err
		}
		writeError(w, http.StatusInternalServerError, "failed to load announcement")
		return nil, err
	}
	role, err := s.stores.Groups.Role(r.Context(), ann.GroupID, uid)
	if err != nil {
		writeError(w, http.StatusForbidden, "you are not a member of this group")
		return nil, err
	}
	if uid != ann.AuthorID && role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only the person who announced it or a group admin can "+action)
		return nil, errForbidden
	}
	return ann, nil
}

// handleUpdateAnnouncementNote edits the note on an open announcement (poster or
// group admin). For a multi-group broadcast the change cascades to every sibling.
func (s *Server) handleUpdateAnnouncementNote(w http.ResponseWriter, r *http.Request) {
	announcementID, ok := parseID(w, r, "announcementID")
	if !ok {
		return
	}
	ann, err := s.authorizeAnnouncementManage(w, r, announcementID, "edit the note")
	if err != nil {
		return
	}
	if ann.Status != models.StatusOpen {
		writeError(w, http.StatusBadRequest, "you can only edit the note of an open announcement")
		return
	}

	var body struct {
		Note string `json:"note"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	note := strings.TrimSpace(body.Note)
	if len(note) > maxNoteLength {
		writeError(w, http.StatusBadRequest, "note is too long")
		return
	}

	affected, err := s.stores.Announcements.UpdateNoteWithSiblings(r.Context(), ann.ID, note)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update note")
		return
	}
	for _, id := range affected {
		s.broadcastAnnouncement(r, id)
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// broadcastAnnouncement reloads an announcement and pushes it to its group room
// and the linked Discord message.
func (s *Server) broadcastAnnouncement(r *http.Request, announcementID int64) {
	announcement, err := s.stores.Announcements.GetByID(r.Context(), announcementID)
	if err != nil {
		return
	}
	s.hub.Broadcast(announcement.GroupID, ws.EventAnnouncementUpdated, announcement)
	s.bot.SyncAnnouncement(r.Context(), announcement)
}
