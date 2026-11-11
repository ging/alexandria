// Package testkit provides a settable fake clock shared by server and client tests.
// Lets tests place the system at any F6 instant without waiting (DP-8).
package testkit

import (
	"sync"

	"github.com/ging/alexandria/internal/core/timex"
)

// FakeClock is a manually driven timex.Clock.
type FakeClock struct {
	mu sync.Mutex
	t  timex.Instant
}

// NewClock starts a fake clock at an RFC 3339 instant.
func NewClock(s string) *FakeClock { return &FakeClock{t: timex.MustInstant(s)} }

// Now returns the current fake instant.
func (c *FakeClock) Now() timex.Instant { c.mu.Lock(); defer c.mu.Unlock(); return c.t }

// Set moves the clock to an RFC 3339 instant.
func (c *FakeClock) Set(s string) { c.mu.Lock(); c.t = timex.MustInstant(s); c.mu.Unlock() }

// Advance moves the clock forward by d.
func (c *FakeClock) Advance(d timex.Duration) { c.mu.Lock(); c.t = c.t.Add(d); c.mu.Unlock() }

// timexParse parses an RFC 3339 instant.
func timexParse(s string) (timex.Instant, error) { return timex.ParseInstant(s) }
