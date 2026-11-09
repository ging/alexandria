// Listing and version-record documents, the §3.4 membership rule and the S4–S6 continuity checks.
// The hub builds listings with Build and the client checks them with CheckContinuity (REQ-HUB-008, REQ-HUB-009, REQ-CLI-004).
package lifecycle

import (
	"bytes"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Document types of §3.4.
const (
	ListingType = "eunomia.versions/v1"
	RecordType  = "eunomia.version/v1"
)

// Listing is the signed part of GET /v1/versions.
type Listing struct {
	Type      string        `json:"type"`
	Space     string        `json:"space"`
	Sequence  uint64        `json:"sequence"`
	IssuedAt  timex.Instant `json:"issued_at"`
	ExpiresAt timex.Instant `json:"expires_at"`
	Versions  []Record      `json:"versions"`
}

// VersionDoc is the signed part of GET /v1/versions/{version}.
type VersionDoc struct {
	Type     string        `json:"type"`
	Space    string        `json:"space"`
	IssuedAt timex.Instant `json:"issued_at"`
	Record   Record        `json:"record"`
}

// Members applies the membership rule: Scheduled, Published or Deprecated at t, or revoked.
func Members(rs []Record, t timex.Instant) []Record {
	var out []Record
	for _, r := range SortAsc(rs) {
		switch StateAt(rs, r, t) {
		case Scheduled, Published, Deprecated:
			out = append(out, r)
		default:
			if r.Revoked() {
				out = append(out, r)
			}
		}
	}
	return out
}

// Build assembles the listing issued at t with sequence seq.
func Build(space string, rs []Record, t timex.Instant, ttl timex.Duration, seq uint64) Listing {
	return Listing{Type: ListingType, Space: space, Sequence: seq, IssuedAt: t, ExpiresAt: t.Add(ttl), Versions: Members(rs, t)}
}

// Accepted is the continuity state kept by the client: last sequence, its bytes digest and known records.
type Accepted struct {
	Sequence uint64            `json:"sequence"`
	Digest   ni.Digest         `json:"digest"`
	Raw      []byte            `json:"raw"`
	Known    map[string]Record `json:"known"`
}

// CheckContinuity applies S4, S5 and S6; identical reports a re-served listing with the same bytes.
func CheckContinuity(prev *Accepted, next Listing, raw []byte) (identical bool, err *codes.Error) {
	if prev == nil {
		return false, nil
	}
	switch {
	case next.Sequence < prev.Sequence:
		return false, codes.New(codes.SyncRollback, "sequence %d is below accepted %d", next.Sequence, prev.Sequence)
	case next.Sequence == prev.Sequence && !bytes.Equal(raw, prev.Raw):
		return false, codes.New(codes.SyncSequenceConflict, "sequence %d served with different bytes", next.Sequence)
	case next.Sequence == prev.Sequence:
		return true, nil
	}
	byVersion := map[string]Record{}
	for _, r := range next.Versions {
		byVersion[r.Version.String()] = r
	}
	for v, k := range prev.Known {
		if k.RevokedFrom == nil {
			continue
		}
		n, ok := byVersion[v]
		if !ok || n.RevokedFrom == nil {
			return false, codes.New(codes.SyncRevocationDropped, "%s lost its revoked_from", v)
		}
	}
	for v, n := range byVersion {
		k, ok := prev.Known[v]
		if !ok {
			continue
		}
		if !k.SameDates(n) || (k.RevokedFrom != nil && *k.RevokedFrom != *n.RevokedFrom) {
			return false, codes.New(codes.SyncDatesChanged, "dates of %s changed", v)
		}
	}
	return false, nil
}

// Merge returns the known records updated with the records of a newly accepted listing.
func Merge(known map[string]Record, l Listing) map[string]Record {
	out := map[string]Record{}
	for k, v := range known {
		out[k] = v
	}
	for _, r := range l.Versions {
		out[r.Version.String()] = r
	}
	return out
}

// KnownSlice returns the known records ordered by version.
func KnownSlice(known map[string]Record) []Record {
	out := make([]Record, 0, len(known))
	for _, r := range known {
		out = append(out, r)
	}
	return SortAsc(out)
}

// NewRevocations returns the versions whose revoked_from appears for the first time in l.
func NewRevocations(prev map[string]Record, l Listing) []semver.Version {
	var out []semver.Version
	for _, r := range l.Versions {
		if r.RevokedFrom == nil {
			continue
		}
		if k, ok := prev[r.Version.String()]; !ok || k.RevokedFrom == nil {
			out = append(out, r.Version)
		}
	}
	return out
}
