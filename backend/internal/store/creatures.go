package store

import (
	"context"
	"strconv"
	"strings"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type CreatureStore struct {
	pool *pgxpool.Pool
}

// Upsert inserts or updates a creature by name. Used by the seeder.
func (s *CreatureStore) Upsert(ctx context.Context, name, difficulty, imageURL string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO creatures (name, difficulty, image_url)
		VALUES ($1, $2, $3)
		ON CONFLICT (name) DO UPDATE
			SET difficulty = EXCLUDED.difficulty,
			    image_url  = EXCLUDED.image_url`,
		name, difficulty, imageURL)
	return err
}

// List returns creatures for a user, optionally filtered by a search term and
// difficulty classes, with each creature's killed state resolved for that user.
func (s *CreatureStore) List(ctx context.Context, userID int64, search string, difficulties []string) ([]models.Creature, error) {
	query := `
		SELECT c.id, c.name, c.difficulty, c.image_url,
		       (wk.user_id IS NOT NULL) AS killed
		FROM creatures c
		LEFT JOIN warden_kills wk ON wk.creature_id = c.id AND wk.user_id = $1
		WHERE 1 = 1`
	args := []any{userID}

	if s := strings.TrimSpace(search); s != "" {
		args = append(args, "%"+s+"%")
		query += " AND c.name ILIKE $2"
	}
	if len(difficulties) > 0 {
		args = append(args, difficulties)
		query += " AND c.difficulty = ANY($" + strconv.Itoa(len(args)) + ")"
	}
	query += " ORDER BY c.name ASC"

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Creature
	for rows.Next() {
		var c models.Creature
		if err := rows.Scan(&c.ID, &c.Name, &c.Difficulty, &c.ImageURL, &c.Killed); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetKilled marks a creature as killed for a user (idempotent).
func (s *CreatureStore) SetKilled(ctx context.Context, userID, creatureID int64) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO warden_kills (user_id, creature_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, creature_id) DO NOTHING`, userID, creatureID)
	return err
}

// UnsetKilled removes a creature's killed mark for a user.
func (s *CreatureStore) UnsetKilled(ctx context.Context, userID, creatureID int64) error {
	_, err := s.pool.Exec(ctx, `
		DELETE FROM warden_kills WHERE user_id = $1 AND creature_id = $2`, userID, creatureID)
	return err
}

// Exists reports whether a creature with the given ID exists.
func (s *CreatureStore) Exists(ctx context.Context, creatureID int64) (bool, error) {
	var exists bool
	err := s.pool.QueryRow(ctx,
		`SELECT EXISTS(SELECT 1 FROM creatures WHERE id = $1)`, creatureID).Scan(&exists)
	return exists, err
}

// KilledIDs returns the creature IDs a user has marked killed on their warden list.
func (s *CreatureStore) KilledIDs(ctx context.Context, userID int64) ([]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT creature_id FROM warden_kills WHERE user_id = $1`, userID)
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

// Highscores returns the statistics leaderboard: every user who has killed or
// announced at least one Warden, with their total kills, the Charm Points those
// kills are worth (weighted by each creature's Bestiary difficulty), and how
// many Wardens they've announced. A single multi-group broadcast counts as one
// announced Warden (its sibling rows share a broadcast_id). Ordering is a
// sensible default; the client re-sorts on demand.
func (s *CreatureStore) Highscores(ctx context.Context) ([]models.HighscoreEntry, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT u.id,
		       COALESCE(NULLIF(u.character_name, ''), u.discord_username) AS name,
		       COALESCE(k.kills, 0)::int         AS kills,
		       COALESCE(k.charm_points, 0)::int  AS charm_points,
		       COALESCE(a.announced, 0)::int     AS announced
		FROM users u
		LEFT JOIN (
			SELECT wk.user_id,
			       COUNT(*) AS kills,
			       SUM(CASE c.difficulty
			             WHEN 'Harmless'    THEN 1
			             WHEN 'Trivial'     THEN 2
			             WHEN 'Easy'        THEN 5
			             WHEN 'Medium'      THEN 10
			             WHEN 'Hard'        THEN 15
			             WHEN 'Challenging' THEN 30
			             ELSE 0 END) AS charm_points
			FROM warden_kills wk
			JOIN creatures c ON c.id = wk.creature_id
			GROUP BY wk.user_id
		) k ON k.user_id = u.id
		LEFT JOIN (
			SELECT author_id,
			       COUNT(DISTINCT COALESCE(broadcast_id, 'id:' || id)) AS announced
			FROM announcements
			GROUP BY author_id
		) a ON a.author_id = u.id
		WHERE COALESCE(k.kills, 0) > 0 OR COALESCE(a.announced, 0) > 0
		ORDER BY kills DESC, charm_points DESC, announced DESC, name ASC
		LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.HighscoreEntry
	for rows.Next() {
		var e models.HighscoreEntry
		if err := rows.Scan(&e.UserID, &e.CharacterName, &e.Kills, &e.CharmPoints, &e.Announced); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// PruneExcept deletes creatures whose name is not in keepNames, but only when
// they have no kill history and are not referenced by any announcement. Returns
// the number of creatures deleted.
func (s *CreatureStore) PruneExcept(ctx context.Context, keepNames []string) (int, error) {
	tag, err := s.pool.Exec(ctx, `
		DELETE FROM creatures c
		WHERE c.name <> ALL($1)
		  AND NOT EXISTS (SELECT 1 FROM warden_kills wk WHERE wk.creature_id = c.id)
		  AND NOT EXISTS (SELECT 1 FROM announcements a WHERE a.creature_id = c.id)`,
		keepNames)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}
