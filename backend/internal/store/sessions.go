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

// AuthByToken returns the user ID and banned flag for a non-expired session
// token in a single query, so requireAuth can reject banned users without an
// extra round-trip.
func (s *SessionStore) AuthByToken(ctx context.Context, token string) (userID int64, banned bool, err error) {
	err = s.pool.QueryRow(ctx, `
		SELECT s.user_id, u.banned
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > now()`, token,
	).Scan(&userID, &banned)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, ErrNotFound
	}
	return userID, banned, err
}

// Delete removes a session token (logout).
func (s *SessionStore) Delete(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	return err
}

// DeleteByUser removes all of a user's sessions (used to force-logout on ban).
func (s *SessionStore) DeleteByUser(ctx context.Context, userID int64) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM sessions WHERE user_id = $1`, userID)
	return err
}
