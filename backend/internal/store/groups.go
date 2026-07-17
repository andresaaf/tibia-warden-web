package store

import (
	"context"
	"errors"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type GroupStore struct {
	pool *pgxpool.Pool
}

// Create makes a new group and adds the creator as owner in a single transaction.
func (s *GroupStore) Create(ctx context.Context, ownerID int64, name, description, visibility string) (*models.Group, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var g models.Group
	err = tx.QueryRow(ctx, `
		INSERT INTO groups (name, description, visibility, owner_id)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, description, visibility, owner_id, created_at`,
		name, description, visibility, ownerID,
	).Scan(&g.ID, &g.Name, &g.Description, &g.Visibility, &g.OwnerID, &g.CreatedAt)
	if err != nil {
		return nil, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES ($1, $2, 'owner')`, g.ID, ownerID); err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}
	g.Role = models.RoleOwner
	g.MemberCount = 1
	return &g, nil
}

// Delete removes a group and (via ON DELETE CASCADE) its members, invites,
// announcements, responses, claims, and link codes.
func (s *GroupStore) Delete(ctx context.Context, groupID int64) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM groups WHERE id = $1`, groupID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetByID returns a group with member count and, if provided, the given user's role.
func (s *GroupStore) GetByID(ctx context.Context, groupID, viewerID int64) (*models.Group, error) {
	var g models.Group
	var role *string
	err := s.pool.QueryRow(ctx, `
		SELECT g.id, g.name, g.description, g.visibility, g.owner_id, g.created_at,
		       g.discord_guild_id, g.discord_channel_id, g.discord_role_id, g.discord_role_name,
		       g.discord_autodelete_seconds,
		       (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
		       (SELECT m.role FROM group_members m WHERE m.group_id = g.id AND m.user_id = $2) AS role
		FROM groups g WHERE g.id = $1`, groupID, viewerID,
	).Scan(&g.ID, &g.Name, &g.Description, &g.Visibility, &g.OwnerID, &g.CreatedAt,
		&g.DiscordGuildID, &g.DiscordChannelID, &g.DiscordRoleID, &g.DiscordRoleName,
		&g.DiscordAutodeleteSeconds, &g.MemberCount, &role)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if role != nil {
		g.Role = *role
	}
	return &g, nil
}

// ListPublic returns public groups matching an optional search term, annotated
// with the viewer's role where they are a member.
func (s *GroupStore) ListPublic(ctx context.Context, viewerID int64, search string) ([]models.Group, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, g.description, g.visibility, g.owner_id, g.created_at,
		       (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
		       (SELECT m.role FROM group_members m WHERE m.group_id = g.id AND m.user_id = $1) AS role
		FROM groups g
		WHERE g.visibility = 'public'
		  AND ($2 = '' OR g.name ILIKE '%' || $2 || '%')
		ORDER BY g.created_at DESC`, viewerID, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}

// ListForUser returns all groups the user is a member of.
func (s *GroupStore) ListForUser(ctx context.Context, userID int64) ([]models.Group, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, g.description, g.visibility, g.owner_id, g.created_at,
		       (SELECT COUNT(*) FROM group_members m WHERE m.group_id = g.id) AS member_count,
		       gm.role
		FROM groups g
		JOIN group_members gm ON gm.group_id = g.id AND gm.user_id = $1
		ORDER BY g.created_at DESC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanGroups(rows)
}

func scanGroups(rows pgx.Rows) ([]models.Group, error) {
	var out []models.Group
	for rows.Next() {
		var g models.Group
		var role *string
		if err := rows.Scan(&g.ID, &g.Name, &g.Description, &g.Visibility, &g.OwnerID, &g.CreatedAt, &g.MemberCount, &role); err != nil {
			return nil, err
		}
		if role != nil {
			g.Role = *role
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

// Role returns the viewer's role in a group, or ErrNotFound if not a member.
func (s *GroupStore) Role(ctx context.Context, groupID, userID int64) (string, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM group_members WHERE group_id = $1 AND user_id = $2`, groupID, userID,
	).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotFound
	}
	return role, err
}

// MemberGroupIDs returns the IDs of all groups the user belongs to.
func (s *GroupStore) MemberGroupIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT group_id FROM group_members WHERE user_id = $1`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// Members lists the members of a group with their display names.
func (s *GroupStore) Members(ctx context.Context, groupID int64) ([]models.GroupMember, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT gm.user_id, u.character_name, u.discord_username, gm.role, gm.joined_at
		FROM group_members gm
		JOIN users u ON u.id = gm.user_id
		WHERE gm.group_id = $1
		ORDER BY
			CASE gm.role WHEN 'owner' THEN 0 WHEN 'admin' THEN 1 ELSE 2 END,
			gm.joined_at ASC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.GroupMember
	for rows.Next() {
		var m models.GroupMember
		if err := rows.Scan(&m.UserID, &m.CharacterName, &m.DiscordName, &m.Role, &m.JoinedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// SetRole updates a member's role. It refuses to change the owner's role.
func (s *GroupStore) SetRole(ctx context.Context, groupID, userID int64, role string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE group_members SET role = $3
		WHERE group_id = $1 AND user_id = $2 AND role <> 'owner'`, groupID, userID, role)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RemoveMember removes a member from a group. The owner cannot be removed.
func (s *GroupStore) RemoveMember(ctx context.Context, groupID, userID int64) error {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM group_members
		WHERE group_id = $1 AND user_id = $2 AND role <> 'owner'`, groupID, userID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CreateInvite creates an invite code for a group. maxUses is nil for unlimited.
func (s *GroupStore) CreateInvite(ctx context.Context, groupID, createdBy int64, code string, expiresAt *time.Time, maxUses *int) (*models.InviteCode, error) {
	var inv models.InviteCode
	err := s.pool.QueryRow(ctx, `
		INSERT INTO invite_codes (group_id, code, created_by, expires_at, max_uses)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, group_id, code, created_by, used_by, used_at, expires_at, created_at, max_uses, use_count`,
		groupID, code, createdBy, expiresAt, maxUses,
	).Scan(&inv.ID, &inv.GroupID, &inv.Code, &inv.CreatedBy, &inv.UsedBy, &inv.UsedAt, &inv.ExpiresAt, &inv.CreatedAt, &inv.MaxUses, &inv.UseCount)
	if err != nil {
		return nil, err
	}
	return &inv, nil
}

// ListInvites returns the invite codes for a group.
func (s *GroupStore) ListInvites(ctx context.Context, groupID int64) ([]models.InviteCode, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, group_id, code, created_by, used_by, used_at, expires_at, created_at, max_uses, use_count
		FROM invite_codes WHERE group_id = $1 ORDER BY created_at DESC`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.InviteCode
	for rows.Next() {
		var inv models.InviteCode
		if err := rows.Scan(&inv.ID, &inv.GroupID, &inv.Code, &inv.CreatedBy, &inv.UsedBy, &inv.UsedAt, &inv.ExpiresAt, &inv.CreatedAt, &inv.MaxUses, &inv.UseCount); err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// DeleteInvite removes an invite code belonging to a group.
func (s *GroupStore) DeleteInvite(ctx context.Context, groupID, inviteID int64) error {
	tag, err := s.pool.Exec(ctx,
		`DELETE FROM invite_codes WHERE id = $1 AND group_id = $2`, inviteID, groupID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// RedeemInvite consumes a valid, unused, unexpired invite code and adds the user
// as a member, all within a single transaction. Returns the group ID joined.
func (s *GroupStore) RedeemInvite(ctx context.Context, userID int64, code string) (int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)

	var groupID int64
	err = tx.QueryRow(ctx, `
		SELECT group_id FROM invite_codes
		WHERE code = $1
		  AND (max_uses IS NULL OR use_count < max_uses)
		  AND (expires_at IS NULL OR expires_at > now())
		FOR UPDATE`, code,
	).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	if err != nil {
		return 0, err
	}

	if _, err = tx.Exec(ctx, `
		UPDATE invite_codes SET use_count = use_count + 1, used_by = $1, used_at = now() WHERE code = $2`,
		userID, code); err != nil {
		return 0, err
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES ($1, $2, 'member')
		ON CONFLICT (group_id, user_id) DO NOTHING`, groupID, userID); err != nil {
		return 0, err
	}

	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return groupID, nil
}

// JoinPublic adds a user to a public group as a member.
func (s *GroupStore) JoinPublic(ctx context.Context, userID, groupID int64) error {
	var visibility string
	err := s.pool.QueryRow(ctx, `SELECT visibility FROM groups WHERE id = $1`, groupID).Scan(&visibility)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if visibility != models.VisibilityPublic {
		return ErrNotFound
	}
	_, err = s.pool.Exec(ctx, `
		INSERT INTO group_members (group_id, user_id, role)
		VALUES ($1, $2, 'member')
		ON CONFLICT (group_id, user_id) DO NOTHING`, groupID, userID)
	return err
}

// SetDiscordLink connects a group to a Discord guild + channel.
func (s *GroupStore) SetDiscordLink(ctx context.Context, groupID int64, guildID, channelID string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE groups SET discord_guild_id = $2, discord_channel_id = $3 WHERE id = $1`,
		groupID, guildID, channelID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearDiscordLink removes a group's Discord channel link (and mention role).
func (s *GroupStore) ClearDiscordLink(ctx context.Context, groupID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE groups
		SET discord_guild_id = '', discord_channel_id = '', discord_role_id = '', discord_role_name = ''
		WHERE id = $1`, groupID)
	return err
}

// DiscordChannel returns the linked guild and channel IDs for a group. Both are
// empty strings when the group is not linked.
func (s *GroupStore) DiscordChannel(ctx context.Context, groupID int64) (guildID, channelID string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT discord_guild_id, discord_channel_id FROM groups WHERE id = $1`, groupID,
	).Scan(&guildID, &channelID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", ErrNotFound
	}
	return guildID, channelID, err
}

// DiscordSettings returns the linked guild, channel, and mention-role IDs for a
// group. Fields are empty strings when unset.
func (s *GroupStore) DiscordSettings(ctx context.Context, groupID int64) (guildID, channelID, roleID string, err error) {
	err = s.pool.QueryRow(ctx,
		`SELECT discord_guild_id, discord_channel_id, discord_role_id FROM groups WHERE id = $1`, groupID,
	).Scan(&guildID, &channelID, &roleID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", "", ErrNotFound
	}
	return guildID, channelID, roleID, err
}

// SetDiscordRole sets the role to @mention on announcements for a group.
func (s *GroupStore) SetDiscordRole(ctx context.Context, groupID int64, roleID, roleName string) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE groups SET discord_role_id = $2, discord_role_name = $3 WHERE id = $1`,
		groupID, roleID, roleName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ClearDiscordRole removes the mention role from a group.
func (s *GroupStore) ClearDiscordRole(ctx context.Context, groupID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE groups SET discord_role_id = '', discord_role_name = '' WHERE id = $1`, groupID)
	return err
}

// SetDiscordAutodelete sets the auto-delete policy (seconds) for a group's
// mirrored Discord messages.
func (s *GroupStore) SetDiscordAutodelete(ctx context.Context, groupID int64, seconds int) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE groups SET discord_autodelete_seconds = $2 WHERE id = $1`, groupID, seconds)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// DiscordAutodelete returns the auto-delete policy (seconds) for a group.
func (s *GroupStore) DiscordAutodelete(ctx context.Context, groupID int64) (int, error) {
	var seconds int
	err := s.pool.QueryRow(ctx,
		`SELECT discord_autodelete_seconds FROM groups WHERE id = $1`, groupID).Scan(&seconds)
	if errors.Is(err, pgx.ErrNoRows) {
		return -1, ErrNotFound
	}
	return seconds, err
}

// CreateDiscordLinkCode stores a one-time link code for a group.
func (s *GroupStore) CreateDiscordLinkCode(ctx context.Context, groupID, createdBy int64, code string, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO discord_link_codes (code, group_id, created_by, expires_at)
		VALUES ($1, $2, $3, $4)`, code, groupID, createdBy, expiresAt)
	return err
}

// ConsumeDiscordLinkCode validates and deletes a link code, returning its group ID.
func (s *GroupStore) ConsumeDiscordLinkCode(ctx context.Context, code string) (int64, error) {
	var groupID int64
	err := s.pool.QueryRow(ctx, `
		DELETE FROM discord_link_codes
		WHERE code = $1 AND expires_at > now()
		RETURNING group_id`, code,
	).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return groupID, err
}

// PeekDiscordLinkCode returns the group a valid (unexpired) link code belongs to
// without consuming it.
func (s *GroupStore) PeekDiscordLinkCode(ctx context.Context, code string) (int64, error) {
	var groupID int64
	err := s.pool.QueryRow(ctx, `
		SELECT group_id FROM discord_link_codes
		WHERE code = $1 AND expires_at > now()`, code,
	).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return groupID, err
}
