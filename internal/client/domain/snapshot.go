// Package domain defines the immutable snapshot of the last accepted listing with its verified packages and head lifecycle parameters.
// Published atomically by Sync and read by the hot path, so a package is used whole or not at all (§4.4, §4.6, REQ-CLI-007).
package domain

import (
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Snapshot is the read-only view used to classify messages.
type Snapshot struct {
	Space     string
	Sequence  uint64
	IssuedAt  timex.Instant
	ExpiresAt timex.Instant
	Records   []lifecycle.Record
	Known     map[string]lifecycle.Record
	Packages  map[ni.Digest]*pkgv.Package
	Params    lifecycle.Params
}

// InForce returns V_t at local instant tL with the lease margin (§4.6).
func (s *Snapshot) InForce(tL timex.Instant) []lifecycle.Record {
	return lifecycle.InForceSet(s.Records, tL, s.Params.LeaseMargin)
}

// Package returns the verified package of a record.
func (s *Snapshot) Package(r lifecycle.Record) *pkgv.Package { return s.Packages[r.Manifest] }

// KnownRecord returns the last known record of a version, even if it left the listing.
func (s *Snapshot) KnownRecord(v string) (lifecycle.Record, bool) {
	r, ok := s.Known[v]
	return r, ok
}

// HeadParams selects the lifecycle of the greatest non-revoked listed version with a package.
func HeadParams(records []lifecycle.Record, pkgs map[ni.Digest]*pkgv.Package) lifecycle.Params {
	var best *lifecycle.Record
	for i := range records {
		r := records[i]
		if r.Revoked() || pkgs[r.Manifest] == nil {
			continue
		}
		if best == nil || semver.Less(best.Version, r.Version) {
			best = &records[i]
		}
	}
	if best == nil {
		for _, r := range records {
			if p := pkgs[r.Manifest]; p != nil {
				return p.SpaceLock.Lifecycle
			}
		}
		return lifecycle.Defaults()
	}
	return pkgs[best.Manifest].SpaceLock.Lifecycle
}
