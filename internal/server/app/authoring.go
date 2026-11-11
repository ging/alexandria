// Package app implements the Authoring use case: drafts with optimistic concurrency, validation and proposal of frozen candidates.
// Candidates copy the compiled artefacts so later draft edits never change them (REQ-HUB-001, REQ-HUB-002, REQ-HUB-018, REQ-HUB-019).
package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"sort"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/domain"
)

// Authoring implements /v1/namespaces/{ns}/drafts*.
type Authoring struct {
	d      Deps
	reader *VocabReader
}

// Findings is a set of validation errors returned to the caller as problem+json.
type Findings []*codes.Error

// Error implements error with the first finding.
func (f Findings) Error() string { return f[0].Error() }

// Report is the body of :validate.
type Report struct {
	Valid    bool            `json:"valid"`
	Class    lifecycle.Class `json:"class,omitempty"`
	Findings []*codes.Error  `json:"findings"`
}

// CreateDraft stores a new draft after N1–N3.
func (a *Authoring) CreateDraft(p *domain.Principal, ns string, doc []byte) (domain.Draft, error) {
	if err := a.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Draft{}, err
	}
	if err := a.check(ns, doc, 3); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Store.CreateDraft(ns, doc)
	if err != nil {
		return domain.Draft{}, err
	}
	if err := a.d.emit(p, domain.EvDraftCreated, ns, d.ID); err != nil {
		return domain.Draft{}, err
	}
	a.d.log().Info("draft created", slog.String("namespace", ns), slog.String("draft_id", d.ID), slog.Int("rev", d.Rev))
	return d, nil
}

// GetDraft returns a draft.
func (a *Authoring) GetDraft(p *domain.Principal, ns, id string) (domain.Draft, error) {
	if err := a.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Store.GetDraft(ns, id)
	return d, notFound(err, "draft "+id)
}

// ReplaceDraft replaces a draft when If-Match matches its current ETag.
func (a *Authoring) ReplaceDraft(p *domain.Principal, ns, id, ifMatch string, doc []byte) (domain.Draft, error) {
	if err := a.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Draft{}, err
	}
	cur, err := a.d.Store.GetDraft(ns, id)
	if err != nil {
		return domain.Draft{}, notFound(err, "draft "+id)
	}
	if ifMatch == "" {
		return domain.Draft{}, codes.New(codes.APIPreconditionRequired, "PUT requires If-Match")
	}
	if cur.ETag != ifMatch {
		return domain.Draft{}, codes.New(codes.APIPreconditionFailed, "If-Match %s does not match %s", ifMatch, cur.ETag)
	}
	if err := a.check(ns, doc, 3); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Store.ReplaceDraft(ns, id, ifMatch, doc)
	if errors.Is(err, ErrPreconditionFailed) {
		return domain.Draft{}, codes.New(codes.APIPreconditionFailed, "draft %s changed concurrently", id)
	}
	if err != nil {
		return domain.Draft{}, notFound(err, "draft "+id)
	}
	if err := a.d.emit(p, domain.EvDraftUpdated, ns, d.ID); err != nil {
		return domain.Draft{}, err
	}
	a.d.log().Info("draft updated", slog.String("namespace", ns), slog.String("draft_id", d.ID), slog.Int("rev", d.Rev))
	return d, nil
}

// Validate runs N1–N5 without effects.
func (a *Authoring) Validate(p *domain.Principal, ns, id string) (Report, error) {
	if err := a.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return Report{}, err
	}
	d, err := a.d.Store.GetDraft(ns, id)
	if err != nil {
		return Report{}, notFound(err, "draft "+id)
	}
	ctx, err := a.context(ns)
	if err != nil {
		return Report{}, err
	}
	f := a.d.Compiler.Validate(d.Doc, 5, ctx)
	r := Report{Valid: len(f) == 0, Findings: append([]*codes.Error{}, f...)}
	if r.Valid {
		if c, err := a.d.Compiler.Compile(d.Doc, ctx); err == nil {
			r.Class = c.Class
		}
	}
	return r, nil
}

// Propose freezes a candidate with artefacts, diff, class and dates planned with P = now.
func (a *Authoring) Propose(p *domain.Principal, ns, id string) (domain.Candidate, error) {
	if err := a.d.authorize(domain.OpNamespace, p, ns); err != nil {
		return domain.Candidate{}, err
	}
	d, err := a.d.Store.GetDraft(ns, id)
	if err != nil {
		return domain.Candidate{}, notFound(err, "draft "+id)
	}
	ctx, err := a.context(ns)
	if err != nil {
		return domain.Candidate{}, err
	}
	if f := a.d.Compiler.Validate(d.Doc, 5, ctx); len(f) > 0 {
		return domain.Candidate{}, Findings(f)
	}
	comp, err := a.d.Compiler.Compile(d.Doc, ctx)
	if err != nil {
		return domain.Candidate{}, err
	}
	comp.Source = d.Doc
	recs, rels, err := a.d.records(ns)
	if err != nil {
		return domain.Candidate{}, err
	}
	now := a.d.Clock.Now()
	planned, perr := lifecycle.Plan(lifecycle.Approval{Version: comp.Version, Class: comp.Class, P: now, Params: comp.Params}, recs)
	if perr != nil {
		return domain.Candidate{}, perr
	}
	planned.ID, planned.Namespace = ctx.Namespace+"/versions/"+comp.Version.String(), ctx.Namespace
	c := domain.Candidate{
		ID: newID(), Namespace: ns, DraftID: id, State: domain.Open, Version: comp.Version, Class: comp.Class,
		Diff: diff(comp, rels, ctx.Latest), Planned: planned, ProposedAt: now, Params: comp.Params, Package: copyCompiled(comp),
	}
	if err := a.d.Store.InsertCandidate(c); err != nil {
		return domain.Candidate{}, err
	}
	if err := a.d.emit(p, domain.EvCandidateProposed, ns, c.ID); err != nil {
		return domain.Candidate{}, err
	}
	a.d.log().Info("candidate proposed", slog.String("namespace", ns), slog.String("candidate_id", c.ID), slog.String("draft_id", id), slog.String("version", c.Version.String()), slog.String("class", string(c.Class)))
	return c, nil
}

// check runs validation up to level and returns findings as an error.
func (a *Authoring) check(ns string, doc []byte, level int) error {
	ctx, err := a.context(ns)
	if err != nil {
		return err
	}
	if f := a.d.Compiler.Validate(doc, level, ctx); len(f) > 0 {
		return Findings(f)
	}
	return nil
}

// context returns the namespace IRI, latest published version and its source, and the vocabulary reader of ns.
func (a *Authoring) context(ns string) (CompileContext, error) {
	_, rels, err := a.d.records(ns)
	if err != nil {
		return CompileContext{}, err
	}
	ctx := CompileContext{Namespace: a.d.nsIRI(ns), Vocab: func(iri string) ([]byte, error) { return a.reader.ByIRI(ns, iri) }}
	for _, r := range rels {
		v := r.Record.Version
		if ctx.Latest == nil || semver.Less(*ctx.Latest, v) {
			ctx.Latest, ctx.Previous = &v, r.Source
		}
	}
	return ctx, nil
}

// diff lists added, removed and changed files against the latest release.
func diff(comp domain.Compiled, rels []domain.Release, latest *semver.Version) []string {
	prev := map[string]ni.Digest{}
	for _, r := range rels {
		if latest != nil && semver.Compare(r.Record.Version, *latest) == 0 {
			for _, e := range r.Entries {
				prev[e.Path] = e.Digest
			}
		}
	}
	out := []string{}
	cur := map[string]bool{}
	for _, e := range comp.Entries {
		cur[e.Path] = true
		switch d, ok := prev[e.Path]; {
		case !ok:
			out = append(out, "added "+e.Path)
		case d != e.Digest:
			out = append(out, "changed "+e.Path)
		}
	}
	for p := range prev {
		if !cur[p] {
			out = append(out, "removed "+p)
		}
	}
	sort.Strings(out)
	return out
}

// copyCompiled deep-copies compiled artefacts so the candidate owns its bytes.
func copyCompiled(c domain.Compiled) domain.Compiled {
	b, _ := json.Marshal(c)
	var out domain.Compiled
	_ = json.Unmarshal(b, &out)
	out.Entries = append([]pkgv.File(nil), c.Entries...)
	return out
}
