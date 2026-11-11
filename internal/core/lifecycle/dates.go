// Package lifecycle implements date planning (§3.3.2 table), approval invariants DATE-1/DATE-2, revocation REV-1..REV-4 and the S7 check.
// Pure functions: the publication instant P and the existing records enter as arguments (REQ-HUB-005, REQ-HUB-006, REQ-HUB-007, REQ-HUB-014, REQ-CLI-004).
package lifecycle

import (
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Approval carries the candidate data and the optional grace_window of :approve.
type Approval struct {
	Version semver.Version
	Class   Class
	Grace   *timex.Duration
	P       timex.Instant
	Params  Params
}

// ClassOf is N4: initial for the first version, breaking on a MAJOR increment, compatible otherwise.
func ClassOf(x semver.Version, latest *semver.Version) (Class, bool) {
	if latest == nil {
		return Initial, true
	}
	if !semver.Less(*latest, x) {
		return "", false
	}
	if x.Major > latest.Major {
		return Breaking, true
	}
	return Compatible, true
}

// Plan computes the record dates of X following the exhaustive table of §3.3.2 (no emergency, minimum grace).
func Plan(a Approval, rs []Record) (Record, *codes.Error) {
	nc := a.P.Add(a.Params.PropagationWindow)
	rec := Record{Version: a.Version, Class: a.Class, PublishedAt: a.P, NewContentFrom: nc}
	switch a.Class {
	case Initial, Compatible:
		if a.Grace != nil {
			return Record{}, codes.New(codes.DatesGraceOnCompatible, "grace_window is only allowed on breaking versions")
		}
		rec.EnforcedFrom = nc
		if a.Class == Compatible {
			if y, ok := predecessor(rs, a.Version); ok && y.EnforcedFrom > nc {
				rec.EnforcedFrom = y.EnforcedFrom
			}
		}
	case Breaking:
		switch {
		case a.Grace == nil:
			rec.EnforcedFrom = nc.Add(a.Params.GraceWindow)
		case *a.Grace >= a.Params.MinGraceWindow:
			rec.EnforcedFrom = nc.Add(*a.Grace)
		default:
			return Record{}, codes.New(codes.DatesGraceTooShort, "grace_window %s is below min_grace_window %s", *a.Grace, a.Params.MinGraceWindow)
		}
	}
	return rec, nil
}

// predecessor returns Y: the greatest published version lower than x without revoked_from.
func predecessor(rs []Record, x semver.Version) (Record, bool) {
	var lower []Record
	for _, r := range rs {
		if semver.Less(r.Version, x) {
			lower = append(lower, r)
		}
	}
	return Greatest(lower, true)
}

// CheckApproval verifies DATE-1 and DATE-2 for the planned record x.
func CheckApproval(x Record, rs []Record, p Params) *codes.Error {
	for _, y := range rs {
		if semver.Less(y.Version, x.Version) && !y.Revoked() && x.EnforcedFrom < y.EnforcedFrom {
			return codes.New(codes.DatesNonMonotonic, "enforced_from %s is before enforced_from %s of %s", x.EnforcedFrom, y.EnforcedFrom, y.Version)
		}
	}
	all := append(append([]Record(nil), rs...), x)
	if n := len(InForceSet(all, x.NewContentFrom, 0)); n > p.MaxConcurrentVersions {
		return codes.New(codes.DatesTooManyVersions, "%d versions in force at %s, maximum %d", n, x.NewContentFrom, p.MaxConcurrentVersions)
	}
	return nil
}

// Revocation carries a revocation request made at instant R.
type Revocation struct {
	Version       semver.Version
	R             timex.Instant
	RevokedFrom   *timex.Instant
	Justification string
	PW            timex.Duration
}

// CheckRevocation applies REV-1..REV-4 in order and returns the effective revoked_from.
func CheckRevocation(in Revocation, rs []Record) (timex.Instant, *codes.Error) {
	v, _ := Find(rs, in.Version)
	if v.Revoked() {
		return 0, codes.New(codes.DatesAlreadyRevoked, "%s is already revoked", in.Version)
	}
	if strings.TrimSpace(in.Justification) == "" {
		return 0, codes.New(codes.DatesJustificationRequired, "a revocation requires a justification")
	}
	rf := in.R.Add(in.PW)
	if in.RevokedFrom != nil {
		rf = *in.RevokedFrom
	}
	if rf < in.R.Add(in.PW) {
		return 0, codes.New(codes.DatesRevocationTooEarly, "revoked_from %s is before R + propagation_window", rf)
	}
	applied := make([]Record, len(rs))
	for i, r := range rs {
		applied[i] = r
		if semver.Compare(r.Version, in.Version) == 0 {
			applied[i].RevokedFrom = &rf
		}
	}
	if len(InForceSet(applied, rf, 0)) == 0 {
		return 0, codes.New(codes.DatesRevokeLeavesNoHead, "no version would be in force at %s", rf)
	}
	return rf, nil
}

// CheckRecords is S7 over a listing; paramsOf returns the manifest lifecycle of each version.
func CheckRecords(rs []Record, paramsOf func(semver.Version) Params) *codes.Error {
	for _, r := range rs {
		p := paramsOf(r.Version)
		switch {
		case r.NewContentFrom.Sub(r.PublishedAt) < p.PropagationWindow:
			return codes.New(codes.SyncDatesInvalid, "%s: new_content_from − published_at is below propagation_window", r.Version)
		case r.EnforcedFrom < r.NewContentFrom:
			return codes.New(codes.SyncDatesInvalid, "%s: enforced_from is before new_content_from", r.Version)
		case r.Class == Initial && r.EnforcedFrom != r.NewContentFrom:
			return codes.New(codes.SyncDatesInvalid, "%s: an initial version must have enforced_from = new_content_from", r.Version)
		case r.Class == Breaking && r.EnforcedFrom.Sub(r.NewContentFrom) < p.MinGraceWindow:
			return codes.New(codes.SyncDatesInvalid, "%s: a breaking version needs at least min_grace_window of coexistence", r.Version)
		}
	}
	var prev *Record
	for _, r := range SortAsc(rs) {
		if r.Revoked() {
			continue
		}
		if prev != nil && r.EnforcedFrom < prev.EnforcedFrom {
			return codes.New(codes.SyncDatesInvalid, "%s: enforced_from decreases with respect to %s", r.Version, prev.Version)
		}
		cur := r
		prev = &cur
	}
	return nil
}
