// Package app holds the two atomic cells of the client: the snapshot published by Sync and the rule set published by Rules.
// Each message loads both once, so it uses one package set and one rule revision (REQ-CLI-007, REQ-CLI-029).
package app

import (
	"sync/atomic"

	"github.com/ging/alexandria/internal/client/domain"
)

// Cell holds the current snapshot.
type Cell struct{ p atomic.Pointer[domain.Snapshot] }

// Load returns the current snapshot, or nil before the first E1.
func (c *Cell) Load() *domain.Snapshot { return c.p.Load() }

// Store publishes a snapshot.
func (c *Cell) Store(s *domain.Snapshot) { c.p.Store(s) }

// RulesCell holds the current rule set.
type RulesCell struct{ p atomic.Pointer[domain.Ruleset] }

// Load returns the current rule set, never nil once the client started.
func (c *RulesCell) Load() *domain.Ruleset {
	if rs := c.p.Load(); rs != nil {
		return rs
	}
	return &domain.Ruleset{Rules: []domain.Rule{}}
}

// Store publishes a rule set.
func (c *RulesCell) Store(rs *domain.Ruleset) { c.p.Store(rs) }
