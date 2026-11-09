// Package domain defines the agreement registry entry and the exhaustive admission table of the agreement_admission gate.
// Registered agreements stay usable until revoked_from + L of their version (§4.9, REQ-CLI-016).
package domain

import (
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/timex"
)

// Entry is one agreement of the registry (§4.9).
type Entry struct {
	AgreementID     string        `json:"agreement_id"`
	ProviderPID     string        `json:"provider_pid"`
	ConsumerPID     string        `json:"consumer_pid"`
	Role            string        `json:"role"`
	Version         string        `json:"version"`
	Manifest        ni.Digest     `json:"manifest"`
	DeclaredProfile *string       `json:"declared_profile"`
	RecordedAt      timex.Instant `json:"recorded_at"`
	Source          string        `json:"source"`
}

// Usable reports whether the agreement version is not revoked at tL with lease L.
func Usable(rec lifecycle.Record, l timex.Duration, tL timex.Instant) bool {
	return rec.RevokedFrom == nil || rec.RevokedFrom.Add(l) > tL
}

// Admit applies the admission table for an agreement and its known version record.
func Admit(e *Entry, rec lifecycle.Record, l timex.Duration, tL timex.Instant, unknown string) Classification {
	if e == nil {
		if unknown == "reject" {
			return violation(codes.AgreementUnknown)
		}
		return Classification{Result: OK, Codes: []codes.Code{codes.AgreementUnknown}, Evaluated: []string{}}
	}
	v := e.Version
	if !Usable(rec, l, tL) {
		return Classification{Result: Violation, Codes: []codes.Code{codes.AgreementVersionRevoked}, Matched: &v, Evaluated: []string{}}
	}
	return Classification{Result: OK, Matched: &v, Evaluated: []string{}}
}
