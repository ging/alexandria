// Package scheduler is a driving adapter that reissues stale listings and re-checks the signing key on the hub clock.
// Ticks on wall time but decides with the hub Clock, so file_clock jumps trigger issuance (REQ-HUB-008, REQ-HUB-015).
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
)

// Run reissues stale listings every tick and re-resolves the hub DID every keyEvery of hub time, until ctx ends.
func Run(ctx context.Context, h *app.Hub, c timex.Clock, tick time.Duration, keyEvery timex.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	lastKey := c.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			now := c.Now()
			if now.Sub(lastKey) >= keyEvery || now < lastKey {
				h.Keys.CheckSigningKey()
				lastKey = now
			}
			if err := h.Listing.Tick(now); err != nil {
				log.Printf("scheduler: %v", err)
			}
		}
	}
}
