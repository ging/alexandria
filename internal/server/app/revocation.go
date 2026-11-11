// Package app implements the Revocation use case: REV-1..REV-4, a re-signed record with revoked_from and a new listing.
// Revocation is the only date change after publication and happens once per version (REQ-HUB-007, REQ-HUB-014).
package app

import (
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/domain"
)

// Revocation implements .../versions/{version}:revoke.
type Revocation struct {
	d       Deps
	keys    *KeyGate
	listing *Listing
}

// RevokeOptions is the :revoke body.
type RevokeOptions struct {
	Justification string         `json:"justification"`
	RevokedFrom   *timex.Instant `json:"revoked_from,omitempty"`
}

// Revoke adds revoked_from to a published version and returns its re-signed record.
func (r *Revocation) Revoke(p *domain.Principal, ns, version string, o RevokeOptions) ([]byte, error) {
	if err := r.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return nil, err
	}
	v, err := semver.Parse(version)
	if err != nil {
		return nil, codes.New(codes.APINotFound, "version %s not found", version)
	}
	recs, rels, err := r.d.records(ns)
	if err != nil {
		return nil, err
	}
	rec, ok := lifecycle.Find(recs, v)
	if !ok {
		return nil, codes.New(codes.APINotFound, "version %s not found", version)
	}
	now := r.d.Clock.Now()
	params := r.d.Defaults
	for _, rel := range rels {
		if semver.Compare(rel.Record.Version, v) == 0 {
			params = rel.Params
		}
	}
	rf, cerr := lifecycle.CheckRevocation(lifecycle.Revocation{Version: v, R: now, RevokedFrom: o.RevokedFrom, Justification: o.Justification, PW: params.PropagationWindow}, recs)
	if cerr != nil {
		return nil, cerr
	}
	rec.RevokedFrom = &rf
	signed, err := r.keys.Sign(r.d.unsigned(vc.TypeVersionRecord, now, nil, rec), now)
	if err != nil {
		return nil, err
	}
	rv := domain.Revocation{Namespace: ns, Version: v, RevokedFrom: rf, Justification: o.Justification, At: now, Principal: p.String(), SignedRec: signed}
	if err := r.d.Store.AppendRevocation(rv); err != nil {
		return nil, err
	}
	if err := r.d.emit(p, domain.EvVersionRevoked, ns, rec.ID); err != nil {
		return nil, err
	}
	_, _ = r.listing.Issue(ns, now)
	return signed, nil
}
