// Composition of the server use cases around shared ports, and helpers that map failures to coded errors.
// Each use case authorizes with the §3.6 matrix and records events for every effect (REQ-HUB-017, REQ-HUB-020).
package app

import (
	"errors"
	"sync"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/domain"
)

// Deps groups the outbound ports and settings shared by the use cases.
type Deps struct {
	Drafts     DraftRepo
	Candidates CandidateRepo
	Releases   ReleaseStore
	Listings   ListingStore
	Blobs      BlobStore
	Log        TransparencyLog
	Signer     Signer
	Compiler   ProfileCompiler
	Events     EventLog
	Clock      timex.Clock
	DID        did.DID
	Defaults   lifecycle.Params
}

// Hub bundles the use cases of vhub-server.
type Hub struct {
	Authoring   *Authoring
	Release     *Release
	Revocation  *Revocation
	Listing     *Listing
	Query       *Query
	Publication *Publication
}

// New wires the use cases over the given ports.
func New(d Deps) *Hub {
	l := &Listing{d: d, mu: &sync.Mutex{}}
	return &Hub{
		Authoring:   &Authoring{d: d},
		Release:     &Release{d: d, listing: l},
		Revocation:  &Revocation{d: d, listing: l},
		Listing:     l,
		Query:       &Query{d: d},
		Publication: &Publication{d: d},
	}
}

// authorize returns API_FORBIDDEN_ROLE when p cannot execute a.
func authorize(p domain.Principal, a domain.Action) *codes.Error {
	if domain.Allowed(p, a) {
		return nil
	}
	return codes.New(codes.ApiForbiddenRole, "%s requires one of %v", a, domain.Matrix[a])
}

// notFound maps a repository miss to API_NOT_FOUND and passes other errors through.
func notFound(err error, what string) error {
	if errors.Is(err, ErrNotFound) {
		return codes.New(codes.ApiNotFound, "%s not found", what)
	}
	return err
}

// emit appends an event; a failing event log is reported to the caller.
func emit(d Deps, typ, principal, subject string) error {
	_, err := d.Events.Append(domain.Event{TS: d.Clock.Now(), Type: typ, Principal: principal, Subject: subject})
	return err
}

// compileContext returns the latest published version and the hub space.
func compileContext(d Deps) (CompileContext, error) {
	rels, err := d.Releases.Releases()
	if err != nil {
		return CompileContext{}, err
	}
	var ctx CompileContext
	for _, r := range rels {
		v := r.Record.Version
		if ctx.Latest == nil || semver.Less(*ctx.Latest, v) {
			ctx.Latest, ctx.Previous = &v, r.Source
		}
		ctx.Space = r.Space
	}
	return ctx, nil
}

// headParams returns the lifecycle of the greatest non-revoked release, or the defaults.
func headParams(d Deps) (lifecycle.Params, string, error) {
	rels, err := d.Releases.Releases()
	if err != nil {
		return lifecycle.Params{}, "", err
	}
	recs, err := d.Releases.Records()
	if err != nil {
		return lifecycle.Params{}, "", err
	}
	g, ok := lifecycle.Greatest(recs, true)
	space := ""
	for _, r := range rels {
		space = r.Space
		if ok && semver.Compare(r.Record.Version, g.Version) == 0 {
			return r.Params, space, nil
		}
	}
	return d.Defaults, space, nil
}
