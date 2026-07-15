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
