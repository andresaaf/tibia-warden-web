package store

import (
	"context"
	"errors"

	"github.com/baz/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AnnouncementStore struct {
	pool *pgxpool.Pool
}

// Create inserts a new announcement and returns the fully hydrated record.
func (s *AnnouncementStore) Create(ctx context.Context, groupID, creatureID, authorID int64, location, note string, goldCost int) (*models.Announcement, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO announcements (group_id, creature_id, author_id, location, note, gold_cost)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`,
		groupID, creatureID, authorID, location, note, goldCost,
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
		SELECT a.id, a.group_id, a.creature_id, c.name, a.author_id, u.character_name,
		       a.location, a.note, a.gold_cost, a.status, a.killed_at, a.created_at, a.discord_message_id
		FROM announcements a
		JOIN creatures c ON c.id = a.creature_id
		JOIN users u ON u.id = a.author_id
		WHERE a.id = $1`, id,
	).Scan(&a.ID, &a.GroupID, &a.CreatureID, &a.CreatureName, &a.AuthorID, &a.AuthorName,
		&a.Location, &a.Note, &a.GoldCost, &a.Status, &a.KilledAt, &a.CreatedAt, &a.DiscordMessageID)
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
		SELECT a.id, a.group_id, a.creature_id, c.name, a.author_id, u.character_name,
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
		if err := rows.Scan(&a.ID, &a.GroupID, &a.CreatureID, &a.CreatureName, &a.AuthorID, &a.AuthorName,
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

// MarkKilled sets an announcement's status to killed. Only the author may do this;
// the caller is responsible for authorization. Returns ErrNotFound if already killed
// or not owned by the author.
func (s *AnnouncementStore) MarkKilled(ctx context.Context, announcementID, authorID int64) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE announcements
		SET status = 'killed', killed_at = now()
		WHERE id = $1 AND author_id = $2 AND status = 'open'`,
		announcementID, authorID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
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
