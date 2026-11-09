// Release use case: approval runs the five ordered steps of REQ-HUB-004; rejection returns a candidate to draft.
// Steps 1–4 change nothing visible until AppendRelease, so any failure leaves /pub and the listing untouched (REQ-HUB-003..006).
package app

import (
	"encoding/json"

	"github.com/ging/alexandria/vhub/internal/core/codes"
	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/ni"
	"github.com/ging/alexandria/vhub/internal/core/semver"
	"github.com/ging/alexandria/vhub/internal/core/timex"
	"github.com/ging/alexandria/vhub/internal/server/domain"
)

// Release implements /v1/candidates/{id}:approve and :reject.
type Release struct {
	d       Deps
	listing *Listing
}

// ApproveOptions is the :approve body.
type ApproveOptions struct {
	GraceWindow   *timex.Duration `json:"grace_window,omitempty"`
	Emergency     *bool           `json:"emergency,omitempty"`
	Justification string          `json:"justification,omitempty"`
}

// Approve publishes a candidate and returns the signed version record.
func (r *Release) Approve(p domain.Principal, id string, o ApproveOptions) ([]byte, error) {
	if err := authorize(p, domain.CandidateApprove); err != nil {
		return nil, err
	}
	c, err := r.d.Candidates.Get(id)
	if err != nil {
		return nil, notFound(err, "candidate "+id)
	}
	if c.State != domain.Open {
		return nil, codes.New(codes.ApiInvalidState, "candidate %s is %s", id, c.State)
	}
	ctx, err := compileContext(r.d)
	if err != nil {
		return nil, err
	}
	if ctx.Latest != nil && !semver.Less(*ctx.Latest, c.Version) {
		return nil, codes.New(codes.ApiInvalidState, "version %s is not greater than published %s", c.Version, *ctx.Latest)
	}
	pInstant := r.d.Clock.Now()
	rec, err := r.steps(c, o, pInstant)
	if err != nil {
		_ = emit(r.d, domain.EvReleaseAborted, p.Name, id)
		if ce, ok := codes.As(err); ok {
			return nil, ce
		}
		return nil, codes.New(codes.ReleaseAborted, "%v", err)
	}
	if err := emit(r.d, domain.EvCandidateApproved, p.Name, id); err != nil {
		return nil, err
	}
	if err := emit(r.d, domain.EvReleasePublished, p.Name, rec.Version.String()); err != nil {
		return nil, err
	}
	// Step 5: a failure here leaves the release stored; the scheduler issues the listing on its next pass.
	_, _ = r.listing.Issue(pInstant)
	return r.listing.signRecord(rec.Version)
}

// steps runs steps 1 to 4 of REQ-HUB-004 and returns the persisted record.
func (r *Release) steps(c domain.Candidate, o ApproveOptions, pInstant timex.Instant) (lifecycle.Record, error) {
	recs, err := r.d.Releases.Records()
	if err != nil {
		return lifecycle.Record{}, err
	}
	rec, cerr := lifecycle.Plan(lifecycle.Approval{
		Version: c.Version, Class: c.Class, Emergency: o.Emergency, Grace: o.GraceWindow,
		Justification: o.Justification, P: pInstant, Params: c.Params,
	}, recs)
	if cerr != nil {
		return lifecycle.Record{}, cerr
	}
	rec.Manifest = c.Manifest
	if cerr := lifecycle.CheckApproval(rec, recs, c.Params); cerr != nil {
		return lifecycle.Record{}, cerr
	}
	sig, err := r.d.Signer.Sign(c.Package.Manifest)
	if err != nil {
		return lifecycle.Record{}, codes.New(codes.ReleaseAborted, "step 2, signing manifest.json: %v", err)
	}
	sigBytes, _ := json.Marshal(sig)
	receipt, err := r.d.Log.Register(c.Manifest)
	if err != nil {
		return lifecycle.Record{}, codes.New(codes.ReleaseAborted, "step 3, transparency log: %v", err)
	}
	for _, b := range c.Package.Files {
		if err := r.d.Blobs.Put(ni.HexSum(b), b); err != nil {
			return lifecycle.Record{}, codes.New(codes.ReleaseAborted, "step 4, blobs: %v", err)
		}
	}
	rel := domain.Release{
		Record: rec, Space: c.Space, Params: c.Params, Manifest: c.Package.Manifest, ManifestSig: sigBytes,
		Receipt: receipt, SpaceLock: c.Package.SpaceLock, Entries: c.Package.Entries,
	}
	if err := r.d.Releases.AppendRelease(rel, c.ID); err != nil {
		return lifecycle.Record{}, codes.New(codes.ReleaseAborted, "step 4, release rows: %v", err)
	}
	return rec, nil
}

// Reject returns an open candidate to draft with a comment.
func (r *Release) Reject(p domain.Principal, id, comment string) (domain.Candidate, error) {
	if err := authorize(p, domain.CandidateReject); err != nil {
		return domain.Candidate{}, err
	}
	c, err := r.d.Candidates.Get(id)
	if err != nil {
		return domain.Candidate{}, notFound(err, "candidate "+id)
	}
	if c.State != domain.Open {
		return domain.Candidate{}, codes.New(codes.ApiInvalidState, "candidate %s is %s", id, c.State)
	}
	if err := r.d.Candidates.Reject(id, comment); err != nil {
		return domain.Candidate{}, err
	}
	c.State, c.Comment = domain.Rejected, comment
	return c, emit(r.d, domain.EvCandidateRejected, p.Name, id)
}
