// Driving adapter that periodically asks the Listing use case to reissue a stale listing.
// Ticks on wall time but decides with the hub Clock, so file_clock jumps trigger issuance (REQ-HUB-008).
package scheduler

import (
	"context"
	"log"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
)

// Run checks every tick whether the listing must be reissued, until ctx ends.
func Run(ctx context.Context, l *app.Listing, c timex.Clock, tick time.Duration) {
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := l.IssueIfStale(c.Now()); err != nil {
				log.Printf("scheduler: %v", err)
			}
		}
	}
}
