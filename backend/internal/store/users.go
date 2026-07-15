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
		RETURNING id, discord_id, discord_username, discord_avatar, character_name, created_at`,
		discordID, username, avatar,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *UserStore) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, discord_id, discord_username, discord_avatar, character_name, created_at
		FROM users WHERE id = $1`, id,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// SetCharacterName updates a user's Tibia character name.
func (s *UserStore) SetCharacterName(ctx context.Context, id int64, name string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		UPDATE users SET character_name = $2
		WHERE id = $1
		RETURNING id, discord_id, discord_username, discord_avatar, character_name, created_at`,
		id, name,
	).Scan(&u.ID, &u.DiscordID, &u.DiscordUsername, &u.DiscordAvatar, &u.CharacterName, &u.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}
