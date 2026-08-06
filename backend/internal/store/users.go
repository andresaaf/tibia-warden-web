package store

import (
	"context"
	"errors"

	"github.com/andresaaf/tibia-warden-web/backend/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrNotFound is returned when a requested row does not exist.
var ErrNotFound = errors.New("not found")

type UserStore struct {
	pool *pgxpool.Pool
}

// UpsertByDiscord inserts a new user or updates the Discord profile fields for an
// existing user, returning the resulting record.
func (s *UserStore) UpsertByDiscord(ctx context.Context, discordID, username, avatar string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (discord_id, discord_username, discord_avatar)
		VALUES ($1, $2, $3)
		ON CONFLICT (discord_id) DO UPDATE
			SET discord_username = EXCLUDED.discord_username,
			    discord_avatar   = EXCLUDED.discord_avatar
		RETURNING id, discord_id, discord_username, discord_avatar, character_name, is_admin, banned, created_at`,
		discordID, username, avatar,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.IsAdmin, &u.Banned, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, discord_id, discord_username, discord_avatar, character_name, is_admin, banned, created_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.IsAdmin, &u.Banned, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ListForAdmin returns users for the admin panel, optionally filtered by a
// case-insensitive match on Discord username or character name. Newest first.
func (s *UserStore) ListForAdmin(ctx context.Context, search string) ([]models.User, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, discord_id, discord_username, discord_avatar, character_name, is_admin, banned, created_at
		FROM users
		WHERE $1 = ''
		   OR discord_username ILIKE '%' || $1 || '%'
		   OR character_name ILIKE '%' || $1 || '%'
		ORDER BY created_at DESC
		LIMIT 200`, search)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.IsAdmin, &u.Banned, &u.CreatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SetBanned sets or clears a user's site-wide ban flag.
func (s *UserStore) SetBanned(ctx context.Context, id int64, banned bool) error {
	ct, err := s.pool.Exec(ctx, `UPDATE users SET banned = $2 WHERE id = $1`, id, banned)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetAdmin grants or revokes a user's site-wide admin flag.
func (s *UserStore) SetAdmin(ctx context.Context, id int64, isAdmin bool) error {
	ct, err := s.pool.Exec(ctx, `UPDATE users SET is_admin = $2 WHERE id = $1`, id, isAdmin)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetCharacterName updates a user's Tibia character name.
func (s *UserStore) SetCharacterName(ctx context.Context, id int64, name string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		UPDATE users SET character_name = $2
		WHERE id = $1
		RETURNING id, discord_id, discord_username, discord_avatar, character_name, is_admin, banned, created_at`,
		id, name,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.IsAdmin, &u.Banned, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
