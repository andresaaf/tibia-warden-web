package store

import (
	"context"
	"errors"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// checkWindow is how long a check stays relevant: a wild echo can respawn 20-30
// minutes after a visit, so after 30 minutes the subarea is "ready" again and
// the check is no longer shown. Mirrors COOLDOWN_MIN in frontend lib/checker.ts.
const checkWindow = `interval '30 minutes'`

// CheckerStore persists the respawn checker: per-user subarea checks and
// favorites. Sharing is a per-membership flag (group_members.share_checks).
type CheckerStore struct {
	pool *pgxpool.Pool
}

// checkerName is the display name used for a checker: character name, falling
// back to the Discord username for users who haven't onboarded.
const checkerName = `COALESCE(NULLIF(u.character_name, ''), u.discord_username)`

// Check records that userID visited subareaID now. A re-check overwrites the
// row; the replaced time is kept in prev_checked_at (if still within the
// window) so Undo can put it back.
func (s *CheckerStore) Check(ctx context.Context, userID, subareaID int64) (*models.SubareaCheck, error) {
	var c models.SubareaCheck
	err := s.pool.QueryRow(ctx, `
		WITH c AS (
			INSERT INTO subarea_checks (subarea_id, user_id, checked_at)
			VALUES ($1, $2, now())
			ON CONFLICT (subarea_id, user_id) DO UPDATE SET
				prev_checked_at = CASE
					WHEN subarea_checks.checked_at > now() - `+checkWindow+`
					THEN subarea_checks.checked_at
				END,
				checked_at = now()
			RETURNING subarea_id, user_id, checked_at
		)
		SELECT c.subarea_id, c.user_id, `+checkerName+`, u.discord_avatar, c.checked_at
		FROM c JOIN users u ON u.id = c.user_id`, subareaID, userID,
	).Scan(&c.SubareaID, &c.UserID, &c.Name, &c.Avatar, &c.CheckedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// Undo reverts userID's latest check of subareaID: the previous check time is
// restored when there is one still within the window, otherwise the check is
// removed. Returns the resulting check, or nil when none remains.
func (s *CheckerStore) Undo(ctx context.Context, userID, subareaID int64) (*models.SubareaCheck, error) {
	var c models.SubareaCheck
	err := s.pool.QueryRow(ctx, `
		WITH c AS (
			UPDATE subarea_checks
			SET checked_at = prev_checked_at, prev_checked_at = NULL
			WHERE subarea_id = $1 AND user_id = $2
			  AND prev_checked_at > now() - `+checkWindow+`
			RETURNING subarea_id, user_id, checked_at
		)
		SELECT c.subarea_id, c.user_id, `+checkerName+`, u.discord_avatar, c.checked_at
		FROM c JOIN users u ON u.id = c.user_id`, subareaID, userID,
	).Scan(&c.SubareaID, &c.UserID, &c.Name, &c.Avatar, &c.CheckedAt)
	if err == nil {
		return &c, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}
	_, err = s.pool.Exec(ctx,
		`DELETE FROM subarea_checks WHERE subarea_id = $1 AND user_id = $2`, subareaID, userID)
	return nil, err
}

// Visible lists the checks within the window that viewerID may see: their own,
// plus those of anyone sharing checks into a group the viewer belongs to.
// Newest first.
func (s *CheckerStore) Visible(ctx context.Context, viewerID int64) ([]models.SubareaCheck, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT sc.subarea_id, sc.user_id, `+checkerName+`, u.discord_avatar, sc.checked_at
		FROM subarea_checks sc
		JOIN users u ON u.id = sc.user_id
		WHERE sc.checked_at > now() - `+checkWindow+`
		  AND (sc.user_id = $1 OR EXISTS (
			SELECT 1
			FROM group_members author
			JOIN group_members viewer ON viewer.group_id = author.group_id
			WHERE author.user_id = sc.user_id AND author.share_checks
			  AND viewer.user_id = $1
		  ))
		ORDER BY sc.checked_at DESC`, viewerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.SubareaCheck{}
	for rows.Next() {
		var c models.SubareaCheck
		if err := rows.Scan(&c.SubareaID, &c.UserID, &c.Name, &c.Avatar, &c.CheckedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// Favorites returns the subarea IDs userID pinned, oldest pin first.
func (s *CheckerStore) Favorites(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT subarea_id FROM subarea_favorites
		WHERE user_id = $1
		ORDER BY created_at ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// AddFavorite pins subareaID for userID (idempotent).
func (s *CheckerStore) AddFavorite(ctx context.Context, userID, subareaID int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO subarea_favorites (user_id, subarea_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, subarea_id) DO NOTHING`, userID, subareaID)
	return err
}

// RemoveFavorite unpins subareaID for userID (idempotent).
func (s *CheckerStore) RemoveFavorite(ctx context.Context, userID, subareaID int64) error {
	_, err := s.pool.Exec(ctx,
		`DELETE FROM subarea_favorites WHERE user_id = $1 AND subarea_id = $2`, userID, subareaID)
	return err
}

// SharingGroupIDs returns the groups userID shares checks with — the rooms a
// check is broadcast to.
func (s *CheckerStore) SharingGroupIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT group_id FROM group_members WHERE user_id = $1 AND share_checks`, userID)
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

// ShareSettings lists every group userID is in with their share flag.
func (s *CheckerStore) ShareSettings(ctx context.Context, userID int64) ([]models.CheckerShare, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT g.id, g.name, gm.share_checks
		FROM group_members gm
		JOIN groups g ON g.id = gm.group_id
		WHERE gm.user_id = $1
		ORDER BY g.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []models.CheckerShare{}
	for rows.Next() {
		var c models.CheckerShare
		if err := rows.Scan(&c.GroupID, &c.Name, &c.Share); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetShare sets whether userID's checks are shared with groupID. Returns
// ErrNotFound if userID is not a member.
func (s *CheckerStore) SetShare(ctx context.Context, groupID, userID int64, share bool) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE group_members SET share_checks = $3
		WHERE group_id = $1 AND user_id = $2`, groupID, userID, share)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
