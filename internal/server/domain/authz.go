// Package domain implements the §3.5 authorization table as a pure function of operation, principal and steward membership.
// The steward list is read on every request by the caller, so a removal takes effect at once (REQ-HUB-017, REQ-HUB-023).
package domain

// Op is one row of the §3.5 authorization table.
type Op int

// Rows of the §3.5 table.
const (
	OpPublic       Op = iota // public reads
	OpAdmin                  // create namespaces, assign and remove stewards, global events
	OpListStewards           // list the stewards of ns
	OpNamespace              // vocabularies, drafts, candidates, revocations and events of ns
)

// Decision is the outcome of Decide.
type Decision int

// Outcomes of Decide.
const (
	Allow Decision = iota
	Unauthenticated
	Forbidden
)

// Decide applies the §3.5 table; p is nil when the request carries no token.
func Decide(op Op, p *Principal, isSteward bool) Decision {
	if op == OpPublic {
		return Allow
	}
	if p == nil {
		return Unauthenticated
	}
	switch op {
	case OpAdmin:
		if p.IsAdmin {
			return Allow
		}
	case OpListStewards:
		if p.IsAdmin || isSteward {
			return Allow
		}
	case OpNamespace:
		if isSteward {
			return Allow
		}
	}
	return Forbidden
}
