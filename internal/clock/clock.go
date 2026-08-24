// Package clock supplies the time to the rest of the application, so that tests
// can drive it instead of sleeping.
package clock

import (
	"sync"
	"time"
)

// Clock reports the current time.
type Clock interface {
	Now() time.Time
}

// System is the real clock.
type System struct{}

// Now returns the current wall-clock time in UTC.
func (System) Now() time.Time { return time.Now().UTC() }

// Fake is a clock that only moves when a test moves it.
type Fake struct {
	mu  sync.Mutex
	now time.Time
}

// NewFake returns a clock stopped at the given instant.
func NewFake(now time.Time) *Fake {
	return &Fake{now: now.UTC()}
}

// Now returns the instant the clock is stopped at.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Advance moves the clock forward.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now = f.now.Add(d)
}
