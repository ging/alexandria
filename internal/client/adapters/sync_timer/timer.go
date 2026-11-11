// Package sync_timer is the driving adapter of the client timers: Sync when due, GC every gc_interval and the state watcher.
// It ticks on wall time but decides with the client Clock, so file_clock jumps take effect (REQ-CLI-003, REQ-CLI-008, REQ-CLI-033).
package sync_timer

import (
	"context"
	"time"

	"github.com/ging/alexandria/internal/client/app"
)

// Run syncs immediately and then checks every tick until ctx ends.
func Run(ctx context.Context, s *app.Sync, gc *app.GC, st *app.Status, tick time.Duration) {
	step := func() {
		s.RunIfDue()
		gc.RunIfDue()
		st.Watch()
	}
	step()
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			step()
		}
	}
}
