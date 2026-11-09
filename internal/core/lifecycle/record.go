// Package lifecycle defines the version record with its class and its four dates, as carried by listings and version records.
// Shared by hub and client so both read the same dates (§3.4, REQ-SYS-004).
package lifecycle

import (
	"sort"

	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Class is the change class computed by N4.
type Class string

// Change classes of §3.3.
const (
	Initial    Class = "initial"
	Compatible Class = "compatible"
	Breaking   Class = "breaking"
)

// Record is the record object of §3.4; RevokedFrom is nil while the version is not revoked.
type Record struct {
	Version        semver.Version `json:"version"`
	Manifest       ni.Digest      `json:"manifest"`
	Class          Class          `json:"class"`
	Emergency      bool           `json:"emergency"`
	PublishedAt    timex.Instant  `json:"published_at"`
	NewContentFrom timex.Instant  `json:"new_content_from"`
	EnforcedFrom   timex.Instant  `json:"enforced_from"`
	RevokedFrom    *timex.Instant `json:"revoked_from"`
}

// Revoked reports whether the record carries revoked_from.
func (r Record) Revoked() bool { return r.RevokedFrom != nil }

// SameDates reports whether two records carry identical published, new-content and enforced dates.
func (r Record) SameDates(o Record) bool {
	return r.PublishedAt == o.PublishedAt && r.NewContentFrom == o.NewContentFrom && r.EnforcedFrom == o.EnforcedFrom
}

// SortAsc orders records by version, lowest first.
func SortAsc(rs []Record) []Record {
	out := append([]Record(nil), rs...)
	sort.Slice(out, func(i, j int) bool { return semver.Less(out[i].Version, out[j].Version) })
	return out
}

// Find returns the record of v.
func Find(rs []Record, v semver.Version) (Record, bool) {
	for _, r := range rs {
		if semver.Compare(r.Version, v) == 0 {
			return r, true
		}
	}
	return Record{}, false
}

// Greatest returns the greatest version in rs, optionally skipping revoked ones.
func Greatest(rs []Record, skipRevoked bool) (Record, bool) {
	var best Record
	found := false
	for _, r := range rs {
		if skipRevoked && r.Revoked() {
			continue
		}
		if !found || semver.Less(best.Version, r.Version) {
			best, found = r, true
		}
	}
	return best, found
}
