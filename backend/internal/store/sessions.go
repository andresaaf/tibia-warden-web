package store

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SessionStore struct {
	pool *pgxpool.Pool
}

// Create stores a new session token for a user with the given expiry.
func (s *SessionStore) Create(ctx context.Context, token string, userID int64, expiresAt time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)`, token, userID, expiresAt)
	return err
}

// UserIDByToken returns the user ID for a non-expired session token.
func (s *SessionStore) UserIDByToken(ctx context.Context, token string) (int64, error) {
	var userID int64
	err := s.pool.QueryRow(ctx, `
		SELECT user_id FROM sessions
		WHERE token = $1 AND expires_at > now()`, token,
	).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrNotFound
	}
	return userID, err
}

// Delete removes a session token (logout).
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}
