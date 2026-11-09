// Package lifecycle implements the pure in-force computation and reference states of a version, with the lease margin as an argument.
// The hub uses margin 0 (§3.5) and the client margin L (§4.6) on the same code (REQ-CLI-009, REQ-SYS-007, REQ-HUB-009).
package lifecycle

import (
	"sort"

	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// State is the reference state of a version (§3.5).
type State string

// Reference states of §3.5.
const (
	Scheduled  State = "Scheduled"
	Published  State = "Published"
	Deprecated State = "Deprecated"
	Superseded State = "Superseded"
	Retired    State = "Retired"
	Revoked    State = "Revoked"
)

// Ends returns ends(W): enforced_from, or ∞ when W was revoked at or before enforced_from.
func Ends(w Record) timex.Instant {
	if w.RevokedFrom == nil || *w.RevokedFrom > w.EnforcedFrom {
		return w.EnforcedFrom
	}
	return timex.Never
}

// InForce reports whether v is in force at t with lease margin m among rs.
func InForce(rs []Record, v Record, t timex.Instant, m timex.Duration) bool {
	if v.NewContentFrom.Add(-m) > t {
		return false
	}
	if v.RevokedFrom != nil && v.RevokedFrom.Add(m) <= t {
		return false
	}
	for _, w := range rs {
		if semver.Less(v.Version, w.Version) && t >= Ends(w).Add(m) {
			return false
		}
	}
	return true
}

// InForceSet returns the versions in force at t with margin m, greatest first.
func InForceSet(rs []Record, t timex.Instant, m timex.Duration) []Record {
	var out []Record
	for _, r := range rs {
		if InForce(rs, r, t, m) {
			out = append(out, r)
		}
	}
	sort.Slice(out, func(i, j int) bool { return semver.Less(out[j].Version, out[i].Version) })
	return out
}

// StateAt evaluates the §3.5 table top-down for version v at reference instant t.
func StateAt(rs []Record, v Record, t timex.Instant) State {
	switch {
	case v.RevokedFrom != nil && *v.RevokedFrom <= t:
		return Revoked
	case t < v.NewContentFrom:
		return Scheduled
	}
	if set := InForceSet(rs, t, 0); InForce(rs, v, t, 0) {
		if semver.Compare(set[0].Version, v.Version) == 0 {
			return Published
		}
		return Deprecated
	}
	var successor *Record
	for i := range rs {
		w := rs[i]
		if semver.Less(v.Version, w.Version) && Ends(w) <= t && (successor == nil || semver.Less(w.Version, successor.Version)) {
			successor = &rs[i]
		}
	}
	if successor != nil && successor.Class == Breaking {
		return Retired
	}
	return Superseded
}

// End returns end(v): the least ends(W) over W greater than v, or Never (§4.7).
func End(rs []Record, v semver.Version) timex.Instant {
	end := timex.Never
	for _, w := range rs {
		if semver.Less(v, w.Version) && Ends(w) < end {
			end = Ends(w)
		}
	}
	return end
}
