// Package lifecycle defines the lifecycle parameters of a SpaceProfile with the defaults of §2.10 and their LIFECYCLE_* invariants.
// The hub validates drafts with them and the client reads them from space-lock.json (REQ-HUB-001).
package lifecycle

import (
	"encoding/json"
	"fmt"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/timex"
)

// Freshness holds the listing time-to-live and the maximum staleness.
type Freshness struct {
	TTL          timex.Duration `json:"ttl"`
	MaxStaleness timex.Duration `json:"max_staleness"`
}

// Params is SpaceProfile.lifecycle; Degraded maps a gate to continue or reject.
type Params struct {
	Freshness             Freshness         `json:"freshness"`
	GraceWindow           timex.Duration    `json:"grace_window"`
	SyncInterval          timex.Duration    `json:"sync_interval"`
	ClockSkew             timex.Duration    `json:"clock_skew"`
	LeaseMargin           timex.Duration    `json:"lease_margin"`
	PropagationWindow     timex.Duration    `json:"propagation_window"`
	MaxConcurrentVersions int               `json:"max_concurrent_versions"`
	Degraded              map[string]string `json:"degraded,omitempty"`
}

// Defaults returns the single source of default values (§2.10, check C4).
func Defaults() Params {
	return Params{
		Freshness:             Freshness{TTL: 24 * timex.Hour, MaxStaleness: 7 * timex.Day},
		GraceWindow:           30 * timex.Day,
		SyncInterval:          6 * timex.Hour,
		ClockSkew:             5 * timex.Second,
		LeaseMargin:           10 * timex.Second,
		PropagationWindow:     2 * timex.Day,
		MaxConcurrentVersions: 4,
	}
}

// ParseOver decodes a partial lifecycle object on top of base, keeping base values for absent fields.
func ParseOver(base Params, raw []byte) (Params, error) {
	p := base
	if len(raw) == 0 || string(raw) == "null" {
		return p, nil
	}
	if err := json.Unmarshal(raw, &p); err != nil {
		return Params{}, fmt.Errorf("lifecycle: %w", err)
	}
	return p, nil
}

// DegradedMode returns the configured degraded behaviour of a gate (continue by default).
func (p Params) DegradedMode(gate string) string {
	if m, ok := p.Degraded[gate]; ok {
		return m
	}
	return "continue"
}

// Validate checks the §2.10 restrictions in table order and returns one error per finding.
func (p Params) Validate() []*codes.Error {
	var out []*codes.Error
	add := func(ok bool, c codes.Code, path, detail string) {
		if !ok {
			out = append(out, &codes.Error{Code: c, Path: path, Detail: detail})
		}
	}
	add(p.Freshness.MaxStaleness > p.Freshness.TTL, codes.LifecycleStalenessLtTTL, "/lifecycle/freshness/max_staleness", "max_staleness must be greater than ttl")
	add(p.GraceWindow > 0, codes.LifecycleGraceZero, "/lifecycle/grace_window", "grace_window must be greater than P0D")
	add(p.SyncInterval <= p.Freshness.TTL/2, codes.LifecycleSyncInterval, "/lifecycle/sync_interval", "sync_interval must be at most ttl/2")
	add(p.ClockSkew > 0, codes.LifecycleClockSkew, "/lifecycle/clock_skew", "clock_skew must be greater than PT0S")
	add(p.LeaseMargin >= p.ClockSkew, codes.LifecycleLease, "/lifecycle/lease_margin", "lease_margin must be at least clock_skew")
	add(p.PropagationWindow >= p.Freshness.TTL+p.LeaseMargin+2*p.ClockSkew, codes.LifecyclePropagation, "/lifecycle/propagation_window", "propagation_window must be at least ttl + lease_margin + 2·clock_skew")
	add(p.MaxConcurrentVersions >= 2, codes.LifecycleMaxVersions, "/lifecycle/max_concurrent_versions", "max_concurrent_versions must be at least 2")
	return out
}
