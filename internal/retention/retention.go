// Package retention removes unstarred Entries once they are older than a
// configured age, so the reading list does not grow without bound. It is the
// only place an Entry is removed without a reader asking, and the only place
// a tombstone is written for the delta contract, per ADR-0004 and the MVP
// PRD's Retention section.
package retention

import (
	"context"
	"log/slog"
	"time"

	"github.com/gabe-santos/yogurt/internal/clock"
	"github.com/gabe-santos/yogurt/internal/store"
)

// defaultTick is how often the schedule wakes to look for expired Entries,
// when the caller has no opinion.
const defaultTick = time.Hour

// Service removes expired Entries on demand and on its own schedule.
type Service struct {
	store  *store.Store
	clock  clock.Clock
	logger *slog.Logger
	maxAge time.Duration
}

// New wires a retention service. maxAge is how old an unstarred Entry may get
// before Cleanup removes it; a non-positive maxAge disables cleanup entirely,
// which Cleanup and Run both honour by removing nothing.
func New(db *store.Store, now clock.Clock, logger *slog.Logger, maxAge time.Duration) *Service {
	return &Service{store: db, clock: now, logger: logger, maxAge: maxAge}
}

// Cleanup removes every unstarred Entry older than maxAge right now, and
// returns how many it removed.
func (s *Service) Cleanup(ctx context.Context) (int, error) {
	return s.store.CleanupExpiredEntries(ctx, s.maxAge, s.clock.Now())
}

// Run cleans up expired Entries on a schedule until ctx is cancelled. tick
// sets how often the schedule wakes; a non-positive tick falls back to
// defaultTick.
func (s *Service) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = defaultTick
	}
	ticker := time.NewTicker(tick)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if _, err := s.Cleanup(ctx); err != nil {
				s.logger.ErrorContext(ctx, "retention cleanup", "error", err)
			}
		}
	}
}
