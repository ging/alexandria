// Package domain provides the mapping from mode and classification to an action, and the most-restrictive combination across gates.
// Implements the two tables of §4.7 (REQ-SYS-007, REQ-CLI-011, REQ-CLI-012).
package domain

// Action is forward, forward-notice or reject.
type Action string

// Actions of §2.4.
const (
	Forward       Action = "forward"
	ForwardNotice Action = "forward-notice"
	Reject        Action = "reject"
)

// rank orders actions from least to most restrictive.
var rank = map[Action]int{Forward: 0, ForwardNotice: 1, Reject: 2}

// Act returns the action of one gate.
func Act(m Mode, r Result) Action {
	if m != Enforce {
		return Forward
	}
	switch r {
	case Warning:
		return ForwardNotice
	case Violation:
		return Reject
	}
	return Forward
}

// Emits reports whether a gate evaluation produces an event: never in observe, otherwise for non-ok or info findings.
func Emits(m Mode, c Classification) bool {
	return m != Observe && (c.Result != OK || len(c.Codes) > 0)
}

// Combine returns the most restrictive action of the enforce gates, or forward if there is none.
func Combine(enforced []Action) Action {
	out := Forward
	for _, a := range enforced {
		if rank[a] > rank[out] {
			out = a
		}
	}
	return out
}
