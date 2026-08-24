// Package autokill automatically marks Echo Warden announcements as killed a
// fixed time after they were revealed, for the common case where the hunt
// happened but nobody remembered to press "Killed". A Warden only lives ~10
// minutes after it spawns, so once the grace period has passed the reveal is
// certainly stale; closing it out keeps feeds, Discord messages and stats tidy
// without manual cleanup.
//
// This only flips the status (cascading across broadcast siblings, exactly like
// a manual kill) and pushes the same live WS + Discord updates. Claiming stays
// manual — auto-kill never ticks anyone's warden list.
package autokill

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/andresaaf/tibia-warden-web/backend/internal/discord"
	"github.com/andresaaf/tibia-warden-web/backend/internal/store"
	"github.com/andresaaf/tibia-warden-web/backend/internal/ws"
)

// interval is how often the sweeper looks for stale announcements. Frequent
// enough that a kill lands close to its deadline, cheap enough to ignore.
const interval = time.Minute

// Sweeper periodically closes out announcements left open past their grace period.
type Sweeper struct {
	stores *store.Stores
	hub    *ws.Hub
	bot    *discord.Bot // may be nil; its methods are nil-safe
	after  time.Duration
}

// New builds a Sweeper that kills announcements still open `after` their reveal.
func New(stores *store.Stores, hub *ws.Hub, bot *discord.Bot, after time.Duration) *Sweeper {
	return &Sweeper{stores: stores, hub: hub, bot: bot, after: after}
}

// Run sweeps once immediately (catching anything left open across a restart)
// and then on a fixed interval until the context is cancelled.
func (s *Sweeper) Run(ctx context.Context) {
	s.sweep(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sweep(ctx)
		}
	}
}

func (s *Sweeper) sweep(ctx context.Context) {
	ids, err := s.stores.Announcements.ExpiredOpen(ctx, time.Now().Add(-s.after))
	if err != nil {
		slog.Error("autokill: failed to query expired announcements", "error", err)
		return
	}
	// A broadcast reveal appears once per group but one kill cascades to all its
	// siblings, so skip any id already closed earlier in this pass.
	done := make(map[int64]bool)
	for _, id := range ids {
		if done[id] {
			continue
		}
		affected, err := s.stores.Announcements.MarkKilledWithSiblings(ctx, id)
		if err != nil {
			// Already killed by a cascade (or gone) — nothing to do.
			if !errors.Is(err, store.ErrNotFound) {
				slog.Error("autokill: failed to mark killed", "announcement", id, "error", err)
			}
			continue
		}
		for _, aid := range affected {
			done[aid] = true
			a, err := s.stores.Announcements.GetByID(ctx, aid)
			if err != nil {
				continue
			}
			s.hub.Broadcast(a.GroupID, ws.EventAnnouncementUpdated, a)
			s.bot.SyncAnnouncement(ctx, a)
			s.bot.OnAnnouncementKilled(ctx, aid)
		}
		slog.Info("autokill: closed stale announcement", "announcement", id, "cascaded", len(affected))
	}
}
