package store

import (
	"context"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AreaStore struct {
	pool *pgxpool.Pool
}

// List returns every area with its creatures, resolving each creature's killed
// state for the given user (LEFT JOIN warden_kills, same as CreatureStore.List).
// Rows are ordered by area then creature name; they are grouped into nested
// Area values preserving that order.
func (s *AreaStore) List(ctx context.Context, userID int64) ([]models.Area, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.name,
		       c.id, c.name, c.difficulty, c.rarity, c.image_url,
		       (wk.user_id IS NOT NULL) AS killed
		FROM areas a
		JOIN area_creatures ac ON ac.area_id = a.id
		JOIN creatures c ON c.id = ac.creature_id
		LEFT JOIN warden_kills wk ON wk.creature_id = c.id AND wk.user_id = $1
		ORDER BY a.sort_order ASC, a.name ASC, c.name ASC`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Area
	byID := map[int64]int{} // area id -> index in out
	for rows.Next() {
		var (
			areaID   int64
			areaName string
			c        models.Creature
		)
		if err := rows.Scan(&areaID, &areaName,
			&c.ID, &c.Name, &c.Difficulty, &c.Rarity, &c.ImageURL, &c.Killed); err != nil {
			return nil, err
		}
		idx, ok := byID[areaID]
		if !ok {
			out = append(out, models.Area{ID: areaID, Name: areaName})
			idx = len(out) - 1
			byID[areaID] = idx
		}
		out[idx].Creatures = append(out[idx].Creatures, c)
	}
	return out, rows.Err()
}

// UpsertArea inserts or updates an area by name and returns its id. Used by the
// seeder.
func (s *AreaStore) UpsertArea(ctx context.Context, name string) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO areas (name)
		VALUES ($1)
		ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
		RETURNING id`, name).Scan(&id)
	return id, err
}

// ReplaceAreaCreatures sets an area's membership to exactly creatureIDs,
// replacing any previous membership so re-seeding is idempotent.
func (s *AreaStore) ReplaceAreaCreatures(ctx context.Context, areaID int64, creatureIDs []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM area_creatures WHERE area_id = $1`, areaID); err != nil {
		return err
	}
	for _, cid := range creatureIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO area_creatures (area_id, creature_id)
			VALUES ($1, $2)
			ON CONFLICT (area_id, creature_id) DO NOTHING`, areaID, cid); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// CreatureIDsByName resolves creature names to their ids. Names with no matching
// creature are simply absent from the returned map (callers can warn on those).
func (s *AreaStore) CreatureIDsByName(ctx context.Context, names []string) (map[string]int64, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, name FROM creatures WHERE name = ANY($1)`, names)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int64{}
	for rows.Next() {
		var (
			id   int64
			name string
		)
		if err := rows.Scan(&id, &name); err != nil {
			return nil, err
		}
		out[name] = id
	}
	return out, rows.Err()
}
