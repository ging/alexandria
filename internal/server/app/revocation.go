// Revocation use case: applies REV-1..REV-4, appends a new signed record and issues a fresh listing.
// The only date that may ever be added to an emitted record is revoked_from (REQ-HUB-007, REQ-HUB-014).
package app

import (
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/domain"
)

// Revocation implements /v1/versions/{version}:revoke.
type Revocation struct {
	d       Deps
	listing *Listing
}

// RevokeOptions is the :revoke body.
type RevokeOptions struct {
	Justification string         `json:"justification"`
	RevokedFrom   *timex.Instant `json:"revoked_from,omitempty"`
}

// Revoke revokes a published version and returns its updated signed record.
func (r *Revocation) Revoke(p domain.Principal, version string, o RevokeOptions) ([]byte, error) {
	if err := authorize(p, domain.VersionsRevoke); err != nil {
		return nil, err
	}
	v, err := semver.Parse(version)
	if err != nil {
		return nil, codes.New(codes.ApiNotFound, "version %s not found", version)
	}
	rel, err := r.d.Releases.Release(v)
	if err != nil {
		return nil, notFound(err, "version "+version)
	}
	recs, err := r.d.Releases.Records()
	if err != nil {
		return nil, err
	}
	now := r.d.Clock.Now()
	rf, cerr := lifecycle.CheckRevocation(lifecycle.Revocation{
		Version: v, R: now, RevokedFrom: o.RevokedFrom, Justification: o.Justification, PW: rel.Params.PropagationWindow,
	}, recs)
	if cerr != nil {
		return nil, cerr
	}
	rec, _ := lifecycle.Find(recs, v)
	rec.RevokedFrom = &rf
	env, err := r.listing.signDoc(rec, now)
	if err != nil {
		return nil, err
	}
	if err := r.d.Releases.AppendRevocation(domain.Revocation{
		Version: v, RevokedFrom: rf, Justification: o.Justification, At: now, Principal: p.Name, Envelope: env,
	}); err != nil {
		return nil, err
	}
	if err := emit(r.d, domain.EvVersionRevoked, p.Name, version); err != nil {
		return nil, err
	}
	if _, err := r.listing.Issue(now); err != nil {
		return nil, err
	}
	return r.listing.signRecord(v)
}
