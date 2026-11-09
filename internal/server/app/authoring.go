// Authoring use case: drafts with optimistic concurrency, validation and proposal of frozen candidates.
// Candidates copy the compiled artefacts so later draft edits never change them (REQ-HUB-001, REQ-HUB-002, REQ-HUB-018, REQ-HUB-019).
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/server/domain"
)

// Authoring implements /v1/drafts*.
type Authoring struct{ d Deps }

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
func (a *Authoring) CreateDraft(p domain.Principal, doc []byte) (domain.Draft, error) {
	if err := authorize(p, domain.DraftCreate); err != nil {
		return domain.Draft{}, err
	}
	if err := a.check(doc, 3); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Drafts.Create(doc)
	if err != nil {
		return domain.Draft{}, err
	}
	return d, emit(a.d, domain.EvDraftCreated, p.Name, d.ID)
}

// GetDraft returns a draft.
func (a *Authoring) GetDraft(p domain.Principal, id string) (domain.Draft, error) {
	if err := authorize(p, domain.DraftGet); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Drafts.Get(id)
	return d, notFound(err, "draft "+id)
}

// ReplaceDraft replaces a draft when If-Match matches its current ETag.
func (a *Authoring) ReplaceDraft(p domain.Principal, id, ifMatch string, doc []byte) (domain.Draft, error) {
	if err := authorize(p, domain.DraftReplace); err != nil {
		return domain.Draft{}, err
	}
	if ifMatch == "" {
		return domain.Draft{}, codes.New(codes.ApiPreconditionRequired, "PUT requires If-Match")
	}
	cur, err := a.d.Drafts.Get(id)
	if err != nil {
		return domain.Draft{}, notFound(err, "draft "+id)
	}
	if cur.ETag != ifMatch {
		return domain.Draft{}, codes.New(codes.ApiPreconditionFailed, "If-Match %s does not match %s", ifMatch, cur.ETag)
	}
	if err := a.check(doc, 3); err != nil {
		return domain.Draft{}, err
	}
	d, err := a.d.Drafts.Replace(id, ifMatch, doc)
	if errors.Is(err, ErrPreconditionFailed) {
		return domain.Draft{}, codes.New(codes.ApiPreconditionFailed, "draft %s changed concurrently", id)
	}
	if err != nil {
		return domain.Draft{}, notFound(err, "draft "+id)
	}
	return d, emit(a.d, domain.EvDraftUpdated, p.Name, d.ID)
}

// Validate runs N1–N5 without effects.
func (a *Authoring) Validate(p domain.Principal, id string) (Report, error) {
	if err := authorize(p, domain.DraftValidate); err != nil {
		return Report{}, err
	}
	d, err := a.d.Drafts.Get(id)
	if err != nil {
		return Report{}, notFound(err, "draft "+id)
	}
	ctx, err := compileContext(a.d)
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
func (a *Authoring) Propose(p domain.Principal, id string) (domain.Candidate, error) {
	if err := authorize(p, domain.DraftPropose); err != nil {
		return domain.Candidate{}, err
	}
	d, err := a.d.Drafts.Get(id)
	if err != nil {
		return domain.Candidate{}, notFound(err, "draft "+id)
	}
	ctx, err := compileContext(a.d)
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
	recs, err := a.d.Releases.Records()
	if err != nil {
		return domain.Candidate{}, err
	}
	now := a.d.Clock.Now()
	planned, perr := lifecycle.Plan(lifecycle.Approval{Version: comp.Version, Class: comp.Class, P: now, Params: comp.Params}, recs)
	if perr != nil {
		return domain.Candidate{}, perr
	}
	digest := ni.Sum(comp.Manifest)
	planned.Manifest = digest
	diff, err := a.diff(comp, ctx)
	if err != nil {
		return domain.Candidate{}, err
	}
	c := domain.Candidate{
		ID: newID(), DraftID: id, State: domain.Open, Space: comp.Space, Version: comp.Version, Class: comp.Class,
		Manifest: digest, Diff: diff, Planned: planned, ProposedAt: now, Params: comp.Params, Package: copyCompiled(comp),
	}
	if err := a.d.Candidates.Insert(c); err != nil {
		return domain.Candidate{}, err
	}
	return c, emit(a.d, domain.EvCandidateProposed, p.Name, c.ID)
}

// check runs validation up to level and returns findings as an error.
func (a *Authoring) check(doc []byte, level int) error {
	ctx, err := compileContext(a.d)
	if err != nil {
		return err
	}
	if f := a.d.Compiler.Validate(doc, level, ctx); len(f) > 0 {
		return Findings(f)
	}
	return nil
}

// diff lists added, removed and changed files against the latest release.
func (a *Authoring) diff(comp domain.Compiled, ctx CompileContext) ([]string, error) {
	prev := map[string]ni.Digest{}
	if ctx.Latest != nil {
		r, err := a.d.Releases.Release(*ctx.Latest)
		if err != nil {
			return nil, err
		}
		for _, e := range r.Entries {
			prev[e.Path] = e.Digest
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
	return out, nil
}

// copyCompiled deep-copies compiled artefacts so the candidate owns its bytes.
func copyCompiled(c domain.Compiled) domain.Compiled {
	b, _ := json.Marshal(c)
	var out domain.Compiled
	_ = json.Unmarshal(b, &out)
	out.Entries = append([]pkgv.File(nil), c.Entries...)
	return out
}

// newID returns a random identifier.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
