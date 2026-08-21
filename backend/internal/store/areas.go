package store

import (
	"context"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

type AreaStore struct {
	pool *pgxpool.Pool
}

// List returns every area with the DISTINCT union of the creatures across its
// subareas, resolving each creature's killed state for the given user (LEFT JOIN
// warden_kills, same as CreatureStore.List). A creature shared by two subareas of
// the same area appears once. Rows are ordered by area then creature name; they
// are grouped into nested Area values preserving that order.
func (s *AreaStore) List(ctx context.Context, userID int64) ([]models.Area, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT a.id, a.name, a.sort_order,
		       c.id, c.name, c.difficulty, c.rarity, c.image_url,
		       (wk.user_id IS NOT NULL) AS killed
		FROM areas a
		JOIN subareas sa ON sa.area_id = a.id
		JOIN subarea_creatures sc ON sc.subarea_id = sa.id
		JOIN creatures c ON c.id = sc.creature_id
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
			areaID    int64
			areaName  string
			areaOrder int
			c         models.Creature
		)
		if err := rows.Scan(&areaID, &areaName, &areaOrder,
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

// ClearAreas removes all areas (cascading to subareas and their creature
// memberships). Area data is seed-derived, so the seeder rebuilds it wholesale.
func (s *AreaStore) ClearAreas(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM areas`)
	return err
}

// UpsertArea inserts or updates an area by name and returns its id. Used by the
// seeder.
func (s *AreaStore) UpsertArea(ctx context.Context, name string, sortOrder int) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO areas (name, sort_order)
		VALUES ($1, $2)
		ON CONFLICT (name) DO UPDATE SET sort_order = EXCLUDED.sort_order
		RETURNING id`, name, sortOrder).Scan(&id)
	return id, err
}

// UpsertSubarea inserts or updates a subarea by (area_id, name) and returns its
// id.
func (s *AreaStore) UpsertSubarea(ctx context.Context, areaID int64, name string, sortOrder int) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `
		INSERT INTO subareas (area_id, name, sort_order)
		VALUES ($1, $2, $3)
		ON CONFLICT (area_id, name) DO UPDATE SET sort_order = EXCLUDED.sort_order
		RETURNING id`, areaID, name, sortOrder).Scan(&id)
	return id, err
}

// ReplaceSubareaCreatures sets a subarea's membership to exactly creatureIDs,
// replacing any previous membership so re-seeding is idempotent.
func (s *AreaStore) ReplaceSubareaCreatures(ctx context.Context, subareaID int64, creatureIDs []int64) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM subarea_creatures WHERE subarea_id = $1`, subareaID); err != nil {
		return err
	}
	for _, cid := range creatureIDs {
		if _, err := tx.Exec(ctx, `
			INSERT INTO subarea_creatures (subarea_id, creature_id)
			VALUES ($1, $2)
			ON CONFLICT (subarea_id, creature_id) DO NOTHING`, subareaID, cid); err != nil {
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
