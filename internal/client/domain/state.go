// Package domain implements the client state machine as a pure function of the last accepted snapshot and the local instant.
// A failed sync never changes the state; E2 is t_l ≥ validUntil and E3 is t_l − validUntil ≥ max_staleness (§4.5, REQ-SYS-008, REQ-CLI-002, REQ-CLI-008, REQ-CLI-018).
package domain

import "github.com/ging/alexandria/internal/core/timex"

// State is a client state of §4.5.
type State string

// States of §4.5.
const (
	Onboarding State = "Onboarding"
	Active     State = "Active"
	Transition State = "Transition"
	Degraded   State = "Degraded"
	FailClosed State = "FailClosed"
)

// StateAt computes the state at tL.
func StateAt(s *Snapshot, tL timex.Instant) State {
	switch {
	case s == nil:
		return Onboarding
	case tL < s.ValidUntil:
		if len(s.InForce(tL)) <= 1 {
			return Active
		}
		return Transition
	case tL.Sub(s.ValidUntil) < s.Params.Freshness.MaxStaleness:
		return Degraded
	}
	return FailClosed
}
