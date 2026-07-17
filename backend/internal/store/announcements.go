package store

import (
	"context"
	"errors"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementStore struct {
	pool *pgxpool.Pool
}

// Create inserts a new announcement and returns the fully hydrated record.
// broadcastID links announcements from one multi-group broadcast (nil = single).
func (s *AnnouncementStore) Create(ctx context.Context, groupID, creatureID, authorID int64, location, note string, goldCost int, broadcastID *string) (*models.Announcement, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO announcements (group_id, creature_id, author_id, location, note, gold_cost, broadcast_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id`,
		groupID, creatureID, authorID, location, note, goldCost, broadcastID,
	).Scan(&id)
	if err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// GetByID returns a single announcement with its responses and claims.
func (s *AnnouncementStore) GetByID(ctx context.Context, id int64) (*models.Announcement, error) {
	var a models.Announcement
	err := s.pool.QueryRow(ctx, `
		SELECT a.id, a.group_id, a.creature_id, c.name, c.image_url, a.author_id, u.character_name,
		       a.location, a.note, a.gold_cost, a.status, a.killed_at, a.created_at, a.discord_message_id, a.broadcast_id
		FROM announcements a
		JOIN creatures c ON c.id = a.creature_id
		JOIN users u ON u.id = a.author_id
		WHERE a.id = $1`, id,
	).Scan(&a.ID, &a.GroupID, &a.CreatureID, &a.CreatureName, &a.CreatureImageURL, &a.AuthorID, &a.AuthorName,
		&a.Location, &a.Note, &a.GoldCost, &a.Status, &a.KilledAt, &a.CreatedAt, &a.DiscordMessageID, &a.BroadcastID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := s.hydrate(ctx, &a); err != nil {
		return nil, err
	}
	return &a, nil
}

// ListByGroup returns recent announcements for a group, newest first.
func (s *AnnouncementStore) ListByGroup(ctx context.Context, groupID int64, limit int) ([]models.Announcement, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.group_id, a.creature_id, c.name, c.image_url, a.author_id, u.character_name,
		       a.location, a.note, a.gold_cost, a.status, a.killed_at, a.created_at, a.discord_message_id
		FROM announcements a
		JOIN creatures c ON c.id = a.creature_id
		JOIN users u ON u.id = a.author_id
		WHERE a.group_id = $1
		ORDER BY a.created_at DESC
		LIMIT $2`, groupID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Announcement
	for rows.Next() {
		var a models.Announcement
		if err := rows.Scan(&a.ID, &a.GroupID, &a.CreatureID, &a.CreatureName, &a.CreatureImageURL, &a.AuthorID, &a.AuthorName,
			&a.Location, &a.Note, &a.GoldCost, &a.Status, &a.KilledAt, &a.CreatedAt, &a.DiscordMessageID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.hydrate(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// ListForUser returns recent announcements across all groups the user belongs to,
// annotated with the group name and the user's role in that group.
func (s *AnnouncementStore) ListForUser(ctx context.Context, userID int64, limit int) ([]models.Announcement, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.group_id, g.name, gm.role, a.creature_id, c.name, c.image_url,
		       a.author_id, u.character_name, a.location, a.note, a.gold_cost, a.status,
		       a.killed_at, a.created_at, a.discord_message_id, a.broadcast_id
		FROM announcements a
		JOIN groups g ON g.id = a.group_id
		JOIN group_members gm ON gm.group_id = a.group_id AND gm.user_id = $1
		JOIN creatures c ON c.id = a.creature_id
		JOIN users u ON u.id = a.author_id
		ORDER BY a.created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Announcement
	for rows.Next() {
		var a models.Announcement
		if err := rows.Scan(&a.ID, &a.GroupID, &a.GroupName, &a.ViewerRole, &a.CreatureID, &a.CreatureName,
			&a.CreatureImageURL, &a.AuthorID, &a.AuthorName, &a.Location, &a.Note, &a.GoldCost,
			&a.Status, &a.KilledAt, &a.CreatedAt, &a.DiscordMessageID, &a.BroadcastID); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i := range out {
		if err := s.hydrate(ctx, &out[i]); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// hydrate loads the responses and claims for an announcement.
func (s *AnnouncementStore) hydrate(ctx context.Context, a *models.Announcement) error {
	a.Responses = []models.AnnouncementResponse{}
	a.Claims = []models.AnnouncementClaim{}

	respRows, err := s.pool.Query(ctx, `
		SELECT r.user_id, COALESCE(NULLIF(u.character_name, ''), u.discord_username), r.status
		FROM announcement_responses r
		JOIN users u ON u.id = r.user_id
		WHERE r.announcement_id = $1
		ORDER BY r.updated_at ASC`, a.ID)
	if err != nil {
		return err
	}
	defer respRows.Close()
	for respRows.Next() {
		var r models.AnnouncementResponse
		if err := respRows.Scan(&r.UserID, &r.CharacterName, &r.Status); err != nil {
			return err
		}
		a.Responses = append(a.Responses, r)
	}
	if err := respRows.Err(); err != nil {
		return err
	}

	claimRows, err := s.pool.Query(ctx, `
		SELECT cl.user_id, COALESCE(NULLIF(u.character_name, ''), u.discord_username)
		FROM announcement_claims cl
		JOIN users u ON u.id = cl.user_id
		WHERE cl.announcement_id = $1
		ORDER BY cl.claimed_at ASC`, a.ID)
	if err != nil {
		return err
	}
	defer claimRows.Close()
	for claimRows.Next() {
		var c models.AnnouncementClaim
		if err := claimRows.Scan(&c.UserID, &c.CharacterName); err != nil {
			return err
		}
		a.Claims = append(a.Claims, c)
	}
	return claimRows.Err()
}

// SetResponse records or updates a user's coming/ready response.
func (s *AnnouncementStore) SetResponse(ctx context.Context, announcementID, userID int64, status string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO announcement_responses (announcement_id, user_id, status, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (announcement_id, user_id) DO UPDATE
			SET status = EXCLUDED.status, updated_at = now()`,
		announcementID, userID, status)
	return err
}

// ClearResponse removes a user's response from an announcement.
func (s *AnnouncementStore) ClearResponse(ctx context.Context, announcementID, userID int64) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM announcement_responses WHERE announcement_id = $1 AND user_id = $2`,
		announcementID, userID)
	return err
}

// MarkKilledWithSiblings marks an announcement killed (if open) and cascades the
// kill to any sibling announcements from the same multi-group broadcast. It
// returns the IDs of all announcements that changed (primary first). Returns
// ErrNotFound if the primary does not exist or is already killed.
func (s *AnnouncementStore) MarkKilledWithSiblings(ctx context.Context, announcementID int64) ([]int64, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var broadcastID *string
	err = tx.QueryRow(ctx, `
		UPDATE announcements SET status = 'killed', killed_at = now()
		WHERE id = $1 AND status = 'open'
		RETURNING broadcast_id`, announcementID,
	).Scan(&broadcastID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	affected := []int64{announcementID}
	if broadcastID != nil && *broadcastID != "" {
		rows, err := tx.Query(ctx, `
			UPDATE announcements SET status = 'killed', killed_at = now()
			WHERE broadcast_id = $1 AND status = 'open' AND id <> $2
			RETURNING id`, *broadcastID, announcementID)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			affected = append(affected, id)
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
		rows.Close()
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return affected, nil
}

// Claim records that a user obtained the Echo Warden benefit for a killed
// announcement and marks the corresponding creature on their warden list, all
// in one transaction. Returns ErrNotFound if the announcement is not killed.
func (s *AnnouncementStore) Claim(ctx context.Context, announcementID, userID int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	var creatureID int64
	var status string
	err = tx.QueryRow(ctx, `
		SELECT creature_id, status FROM announcements WHERE id = $1`, announcementID,
	).Scan(&creatureID, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != models.StatusKilled {
		return ErrNotFound
	}

	if _, err = tx.Exec(ctx, `
		INSERT INTO announcement_claims (announcement_id, user_id)
		VALUES ($1, $2)
		ON CONFLICT (announcement_id, user_id) DO NOTHING`, announcementID, userID); err != nil {
		return err
	}

	// Only update the warden list for users with a real (onboarded) account.
	// Discord-only participants (no character name) are recorded on the post but
	// their personal list is left untouched.
	if _, err = tx.Exec(ctx, `
		INSERT INTO warden_kills (user_id, creature_id)
		SELECT $1, $2
		WHERE EXISTS (SELECT 1 FROM users WHERE id = $1 AND character_name <> '')
		ON CONFLICT (user_id, creature_id) DO NOTHING`, userID, creatureID); err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GroupID returns the group an announcement belongs to.
func (s *AnnouncementStore) GroupID(ctx context.Context, announcementID int64) (int64, error) {
	var groupID int64
	err := s.pool.QueryRow(ctx,
		`SELECT group_id FROM announcements WHERE id = $1`, announcementID).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return groupID, err
}

// SetDiscordMessageID records the mirrored Discord message ID for an announcement.
func (s *AnnouncementStore) SetDiscordMessageID(ctx context.Context, announcementID int64, messageID string) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE announcements SET discord_message_id = $2 WHERE id = $1`, announcementID, messageID)
	return err
}

// ScheduleDiscordDelete marks an announcement's Discord message for deletion at a time.
func (s *AnnouncementStore) ScheduleDiscordDelete(ctx context.Context, announcementID int64, at time.Time) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE announcements SET discord_delete_at = $2 WHERE id = $1`, announcementID, at)
	return err
}

// ClearDiscordMessage forgets the mirrored message (after it has been deleted).
func (s *AnnouncementStore) ClearDiscordMessage(ctx context.Context, announcementID int64) error {
	_, err := s.pool.Exec(ctx,
		`UPDATE announcements SET discord_message_id = '', discord_delete_at = NULL WHERE id = $1`, announcementID)
	return err
}

// DueDiscordDelete identifies a mirrored message that is due for deletion.
type DueDiscordDelete struct {
	AnnouncementID int64
	GroupID        int64
	MessageID      string
}

// DueDiscordDeletes returns mirrored messages whose scheduled delete time has passed.
func (s *AnnouncementStore) DueDiscordDeletes(ctx context.Context) ([]DueDiscordDelete, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, group_id, discord_message_id
		FROM announcements
		WHERE discord_message_id <> '' AND discord_delete_at IS NOT NULL AND discord_delete_at <= now()
		LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DueDiscordDelete
	for rows.Next() {
		var d DueDiscordDelete
		if err := rows.Scan(&d.AnnouncementID, &d.GroupID, &d.MessageID); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}
