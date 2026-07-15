package api

import (
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/baz/tibia-warden-web/backend/internal/models"
	"github.com/baz/tibia-warden-web/backend/internal/store"
)

// handleListGroups returns public groups plus the groups the user belongs to.
func (s *Server) handleListGroups(w http.ResponseWriter, r *http.Request) {
	uid := userID(r)
	scope := r.URL.Query().Get("scope")

	if scope == "mine" {
		groups, err := s.stores.Groups.ListForUser(r.Context(), uid)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to load groups")
			return
		}
		writeJSON(w, http.StatusOK, orEmptyGroups(groups))
		return
	}

	groups, err := s.stores.Groups.ListPublic(r.Context(), uid, strings.TrimSpace(r.URL.Query().Get("search")))
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load groups")
		return
	}
	writeJSON(w, http.StatusOK, orEmptyGroups(groups))
}

// handleCreateGroup creates a new group owned by the current user.
func (s *Server) handleCreateGroup(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Visibility  string `json:"visibility"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	name := strings.TrimSpace(body.Name)
	if name == "" || len(name) > 80 {
		writeError(w, http.StatusBadRequest, "group name must be between 1 and 80 characters")
		return
	}
	visibility := body.Visibility
	if visibility != models.VisibilityPublic && visibility != models.VisibilityPrivate {
		visibility = models.VisibilityPublic
	}

	group, err := s.stores.Groups.Create(r.Context(), userID(r), name, strings.TrimSpace(body.Description), visibility)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create group")
		return
	}
	writeJSON(w, http.StatusCreated, group)
}

// handleGetGroup returns a single group the user can see (public or a member).
func (s *Server) handleGetGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	group, err := s.stores.Groups.GetByID(r.Context(), groupID, userID(r))
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	if group.Visibility == models.VisibilityPrivate && group.Role == "" {
		writeError(w, http.StatusForbidden, "you are not a member of this group")
		return
	}
	writeJSON(w, http.StatusOK, group)
}

// handleJoinPublic adds the current user to a public group.
func (s *Server) handleJoinPublic(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if err := s.stores.Groups.JoinPublic(r.Context(), userID(r), groupID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "public group not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to join group")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

// handleRedeemInvite joins a group via a one-time invite code.
func (s *Server) handleRedeemInvite(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Code string `json:"code"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	code := strings.TrimSpace(body.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}
	groupID, err := s.stores.Groups.RedeemInvite(r.Context(), userID(r), code)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "invalid or already-used invite code")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to redeem invite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"groupId": groupID})
}

// handleLeaveGroup removes the current user from a group (owners cannot leave).
func (s *Server) handleLeaveGroup(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if err := s.stores.Groups.RemoveMember(r.Context(), groupID, userID(r)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusBadRequest, "owners cannot leave their own group")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to leave group")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "left"})
}

// handleListMembers lists members of a group the user belongs to.
func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	if _, err := s.requireMembership(r, groupID); err != nil {
		writeMembershipError(w, err)
		return
	}
	members, err := s.stores.Groups.Members(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load members")
		return
	}
	if members == nil {
		members = []models.GroupMember{}
	}
	writeJSON(w, http.StatusOK, members)
}

// handleSetMemberRole promotes or demotes a member (owner/admin only).
func (s *Server) handleSetMemberRole(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	targetID, err := strconv.ParseInt(chiURLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can change roles")
		return
	}

	var body struct {
		Role string `json:"role"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if body.Role != models.RoleAdmin && body.Role != models.RoleMember {
		writeError(w, http.StatusBadRequest, "role must be 'admin' or 'member'")
		return
	}
	if err := s.stores.Groups.SetRole(r.Context(), groupID, targetID, body.Role); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "member not found or cannot be modified")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to update role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// handleRemoveMember kicks a member (owner/admin only).
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	targetID, err := strconv.ParseInt(chiURLParam(r, "userID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can remove members")
		return
	}
	if err := s.stores.Groups.RemoveMember(r.Context(), groupID, targetID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "member not found or cannot be removed")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to remove member")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "removed"})
}

// handleListInvites lists invite codes for a group (owner/admin only).
func (s *Server) handleListInvites(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can view invites")
		return
	}
	invites, err := s.stores.Groups.ListInvites(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load invites")
		return
	}
	if invites == nil {
		invites = []models.InviteCode{}
	}
	writeJSON(w, http.StatusOK, invites)
}

// handleCreateInvite issues a new one-time invite code (owner/admin only).
func (s *Server) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can create invites")
		return
	}

	var body struct {
		ExpiresInHours int `json:"expiresInHours"`
	}
	// Body is optional; ignore decode errors on empty bodies.
	_ = decodeJSON(r, &body)

	var expiresAt *time.Time
	if body.ExpiresInHours > 0 {
		t := time.Now().Add(time.Duration(body.ExpiresInHours) * time.Hour)
		expiresAt = &t
	}

	code, err := randomInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate invite code")
		return
	}
	invite, err := s.stores.Groups.CreateInvite(r.Context(), groupID, userID(r), code, expiresAt)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}
	writeJSON(w, http.StatusCreated, invite)
}

// handleDeleteInvite removes an invite code (owner/admin only).
func (s *Server) handleDeleteInvite(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	inviteID, err := strconv.ParseInt(chiURLParam(r, "inviteID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite id")
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can delete invites")
		return
	}
	if err := s.stores.Groups.DeleteInvite(r.Context(), groupID, inviteID); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "invite not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "failed to delete invite")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// requireMembership resolves the user's role in a group or returns ErrNotFound.
func (s *Server) requireMembership(r *http.Request, groupID int64) (string, error) {
	return s.stores.Groups.Role(r.Context(), groupID, userID(r))
}

func writeMembershipError(w http.ResponseWriter, err error) {
	if errors.Is(err, store.ErrNotFound) {
		writeError(w, http.StatusForbidden, "you are not a member of this group")
		return
	}
	writeError(w, http.StatusInternalServerError, "failed to verify membership")
}

func parseID(w http.ResponseWriter, r *http.Request, key string) (int64, bool) {
	id, err := strconv.ParseInt(chiURLParam(r, key), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid "+key)
		return 0, false
	}
	return id, true
}

func orEmptyGroups(groups []models.Group) []models.Group {
	if groups == nil {
		return []models.Group{}
	}
	return groups
}

func randomInviteCode() (string, error) {
	b := make([]byte, 10)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return strings.ToUpper(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(b)), nil
}

// handleCreateDiscordLinkCode issues a short-lived code an owner/admin runs via
// the bot's /link command in the target channel (owner/admin only).
func (s *Server) handleCreateDiscordLinkCode(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can link Discord")
		return
	}
	code, err := randomInviteCode()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to generate code")
		return
	}
	expiresAt := time.Now().Add(15 * time.Minute)
	if err := s.stores.Groups.CreateDiscordLinkCode(r.Context(), groupID, userID(r), code, expiresAt); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create link code")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"code":      code,
		"expiresAt": expiresAt,
	})
}

// handleUnlinkDiscord disconnects a group from its Discord channel (owner/admin only).
func (s *Server) handleUnlinkDiscord(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can unlink Discord")
		return
	}
	if err := s.stores.Groups.ClearDiscordLink(r.Context(), groupID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to unlink Discord")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unlinked"})
}

// handleListDiscordRoles returns the assignable roles of a group's linked guild
// (owner/admin only) so one can be chosen to @mention on announcements.
func (s *Server) handleListDiscordRoles(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can manage Discord")
		return
	}
	guildID, _, _, err := s.stores.Groups.DiscordSettings(r.Context(), groupID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load group")
		return
	}
	if guildID == "" {
		writeError(w, http.StatusBadRequest, "link a Discord channel first")
		return
	}
	if s.bot == nil {
		writeError(w, http.StatusBadRequest, "the Discord bot is not enabled")
		return
	}
	roles, err := s.bot.GuildRoles(guildID)
	if err != nil {
		writeError(w, http.StatusBadGateway, "failed to fetch roles from Discord")
		return
	}
	if roles == nil {
		roles = []models.DiscordRole{}
	}
	writeJSON(w, http.StatusOK, roles)
}

// handleSetDiscordRole sets the role to @mention on announcements (owner/admin only).
func (s *Server) handleSetDiscordRole(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can manage Discord")
		return
	}
	var body struct {
		RoleID   string `json:"roleId"`
		RoleName string `json:"roleName"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	roleID := strings.TrimSpace(body.RoleID)
	if roleID == "" {
		writeError(w, http.StatusBadRequest, "a role is required")
		return
	}
	if err := s.stores.Groups.SetDiscordRole(r.Context(), groupID, roleID, strings.TrimSpace(body.RoleName)); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to set role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "set"})
}

// handleClearDiscordRole removes the announcement mention role (owner/admin only).
func (s *Server) handleClearDiscordRole(w http.ResponseWriter, r *http.Request) {
	groupID, ok := parseID(w, r, "groupID")
	if !ok {
		return
	}
	role, err := s.requireMembership(r, groupID)
	if err != nil {
		writeMembershipError(w, err)
		return
	}
	if role != models.RoleOwner && role != models.RoleAdmin {
		writeError(w, http.StatusForbidden, "only owners and admins can manage Discord")
		return
	}
	if err := s.stores.Groups.ClearDiscordRole(r.Context(), groupID); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to clear role")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cleared"})
}
