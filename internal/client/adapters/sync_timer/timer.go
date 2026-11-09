// Package sync_timer is a driving adapter that ticks on wall time, runs Sync when due by the client Clock and watches state changes.
// Syncs at start and then within sync_interval of the head package (REQ-CLI-003, REQ-CLI-027).
package sync_timer

import (
	"context"
	"time"

	"github.com/ging/alexandria/internal/client/app"
)

// Run syncs immediately and then checks every tick until ctx ends.
func Run(ctx context.Context, s *app.Sync, st *app.Status, tick time.Duration) {
	s.RunIfDue()
	st.Watch()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.RunIfDue()
			st.Watch()
		}
	}
}
