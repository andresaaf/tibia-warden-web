package store

import "github.com/jackc/pgx/v5/pgxpool"

// Stores aggregates all repository types over a shared connection pool.
type Stores struct {
	pool          *pgxpool.Pool
	Users         *UserStore
	Sessions      *SessionStore
	Creatures     *CreatureStore
	Groups        *GroupStore
	Announcements *AnnouncementStore
}

// New constructs the repository set bound to the given pool.
func New(pool *pgxpool.Pool) *Stores {
	return &Stores{
		pool:          pool,
		Users:         &UserStore{pool: pool},
		Sessions:      &SessionStore{pool: pool},
		Creatures:     &CreatureStore{pool: pool},
		Groups:        &GroupStore{pool: pool},
		Announcements: &AnnouncementStore{pool: pool},
	}
}
