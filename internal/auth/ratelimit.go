package auth

import (
	"sync"
	"time"

	"github.com/gabe-santos/yogurt/internal/clock"
)

// MaxLoginFailures is how many failed logins a client may make before it is
// blocked, and LoginFailureWindow is how long the block lasts. A single reader
// mistypes a password a handful of times; a brute-force attempt does not stop.
const (
	MaxLoginFailures   = 5
	LoginFailureWindow = 15 * time.Minute
)

// Limiter blocks a client that keeps failing to log in. State is in memory: a
// restart forgets it, which is acceptable because a restart is not something an
// attacker can cause.
type Limiter struct {
	clock  clock.Clock
	max    int
	window time.Duration

	mu       sync.Mutex
	failures map[string]failureRecord
}

type failureRecord struct {
	count int
	last  time.Time
}

// NewLimiter returns a limiter over the configured thresholds.
func NewLimiter(c clock.Clock) *Limiter {
	return &Limiter{
		clock:    c,
		max:      MaxLoginFailures,
		window:   LoginFailureWindow,
		failures: make(map[string]failureRecord),
	}
}

// Blocked reports whether a client is currently blocked, and for how much
// longer.
func (l *Limiter) Blocked(key string) (time.Duration, bool) {
	now := l.clock.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	record, ok := l.failures[key]
	if !ok {
		return 0, false
	}
	until := record.last.Add(l.window)
	if !now.Before(until) {
		delete(l.failures, key)
		return 0, false
	}
	if record.count < l.max {
		return 0, false
	}
	return until.Sub(now), true
}

// Failed records one failed login attempt.
func (l *Limiter) Failed(key string) {
	now := l.clock.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	l.forgetStale(now)

	record := l.failures[key]
	record.count++
	record.last = now
	l.failures[key] = record
}

// Succeeded forgets a client's failures.
func (l *Limiter) Succeeded(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// forgetStale drops records whose window has passed, so that a flood of
// distinct clients cannot grow the map without bound.
func (l *Limiter) forgetStale(now time.Time) {
	for key, record := range l.failures {
		if !now.Before(record.last.Add(l.window)) {
			delete(l.failures, key)
		}
	}
}
