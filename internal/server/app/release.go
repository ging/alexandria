// Package app implements the Release use case: approval runs the ordered steps of REQ-HUB-004; rejection returns a candidate to draft.
// Steps 1–3 change nothing visible until AppendRelease, so any failure leaves /pub and the listing untouched (REQ-HUB-003..006).
package app

import (
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/domain"
)

// Release implements .../candidates/{id}:approve and :reject.
type Release struct {
	d       Deps
	keys    *KeyGate
	listing *Listing
}

// ApproveOptions is the :approve body.
type ApproveOptions struct {
	GraceWindow *timex.Duration `json:"grace_window,omitempty"`
}

// Approve publishes a candidate and returns the signed version record.
func (r *Release) Approve(p *domain.Principal, ns, id string, o ApproveOptions) ([]byte, error) {
	if err := r.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return nil, err
	}
	c, err := r.d.Store.GetCandidate(ns, id)
	if err != nil {
		return nil, notFound(err, "candidate "+id)
	}
	if c.State != domain.Open {
		return nil, codes.New(codes.APIInvalidState, "candidate %s is %s", id, c.State)
	}
	recs, _, err := r.d.records(ns)
	if err != nil {
		return nil, err
	}
	for _, x := range recs {
		if !semver.Less(x.Version, c.Version) {
			return nil, codes.New(codes.APIInvalidState, "version %s is not greater than published %s", c.Version, x.Version)
		}
	}
	pInstant := r.d.Clock.Now()
	rel, err := r.steps(ns, c, o, recs, pInstant)
	if err != nil {
		_ = r.d.emit(p, domain.EvReleaseAborted, ns, id)
		if ce, ok := codes.As(err); ok {
			return nil, ce
		}
		return nil, codes.New(codes.ReleaseAborted, "%v", err)
	}
	if err := r.d.emit(p, domain.EvCandidateApproved, ns, id); err != nil {
		return nil, err
	}
	if err := r.d.emit(p, domain.EvReleasePublished, ns, rel.Record.ID); err != nil {
		return nil, err
	}
	// Step 4: a failure here leaves the release stored; the scheduler issues the listing on its next pass.
	_, _ = r.listing.Issue(ns, pInstant)
	return rel.SignedRec, nil
}

// steps runs steps 1 to 3 of REQ-HUB-004 and returns the persisted release.
func (r *Release) steps(ns string, c domain.Candidate, o ApproveOptions, recs []lifecycle.Record, pInstant timex.Instant) (domain.Release, error) {
	rec, cerr := lifecycle.Plan(lifecycle.Approval{Version: c.Version, Class: c.Class, Grace: o.GraceWindow, P: pInstant, Params: c.Params}, recs)
	if cerr != nil {
		return domain.Release{}, cerr
	}
	if cerr := lifecycle.CheckApproval(rec, recs, c.Params); cerr != nil {
		return domain.Release{}, cerr
	}
	iri := r.d.nsIRI(ns)
	subject := pkgv.Manifest{
		ID: iri + "/versions/" + c.Version.String() + "/manifest", Namespace: iri, Version: c.Version.String(),
		Lifecycle: c.Params, Profiles: c.Package.Profiles, Files: c.Package.Entries,
	}
	manifest, err := r.keys.Sign(r.d.unsigned(vc.TypePackageManifest, pInstant, nil, subject), pInstant)
	if err != nil {
		return domain.Release{}, err
	}
	rec.ID, rec.Namespace, rec.Manifest = iri+"/versions/"+c.Version.String(), iri, ni.Sum(manifest)
	signedRec, err := r.keys.Sign(r.d.unsigned(vc.TypeVersionRecord, pInstant, nil, rec), pInstant)
	if err != nil {
		return domain.Release{}, err
	}
	for _, b := range c.Package.Files {
		if err := r.d.Blobs.Put(ni.HexSum(b), b); err != nil {
			return domain.Release{}, codes.New(codes.ReleaseAborted, "step 3, blobs: %v", err)
		}
	}
	rel := domain.Release{Namespace: ns, Record: rec, Params: c.Params, Manifest: manifest, SignedRec: signedRec, Entries: c.Package.Entries, Source: c.Package.Source}
	if err := r.d.Store.AppendRelease(rel, c.ID); err != nil {
		return domain.Release{}, codes.New(codes.ReleaseAborted, "step 3, release rows: %v", err)
	}
	return rel, nil
}

// Reject returns an open candidate to draft with a comment.
func (r *Release) Reject(p *domain.Principal, ns, id, comment string) (domain.Candidate, error) {
	if err := r.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Candidate{}, err
	}
	c, err := r.d.Store.GetCandidate(ns, id)
	if err != nil {
		return domain.Candidate{}, notFound(err, "candidate "+id)
	}
	if c.State != domain.Open {
		return domain.Candidate{}, codes.New(codes.APIInvalidState, "candidate %s is %s", id, c.State)
	}
	if err := r.d.Store.RejectCandidate(ns, id, comment); err != nil {
		return domain.Candidate{}, err
	}
	c.State, c.Comment = domain.Rejected, comment
	return c, r.d.emit(p, domain.EvCandidateRejected, ns, id)
}
