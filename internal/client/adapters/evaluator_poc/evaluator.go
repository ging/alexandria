// Package evaluator_poc implements the PoC evaluator: runs json-schema artefacts offline and, for SHACL slots, expands the message with json-gold restricted to the manifest contexts.
// SHACL shapes themselves are not run (POC_EVAL_UNSUPPORTED_ROLE info event); the §4.7 slot algorithm is kept (DP-7, REQ-CLI-010).
package evaluator_poc

import (
	"sync"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/jsonld"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
)

// Evaluator compiles package schemas once and validates bodies without network.
type Evaluator struct {
	Events    app.EventSink
	Clock     timex.Clock
	Namespace string
	cache     sync.Map
	loaders   sync.Map
	warned    sync.Map
}

var _ app.Evaluator = (*Evaluator)(nil)

// compiled returns the compiled schema of a package file.
func (e *Evaluator) compiled(p *pkgv.Package, f pkgv.File) (*schemas.Schema, error) {
	key := string(p.Digest) + "|" + f.Path
	if s, ok := e.cache.Load(key); ok {
		return s.(*schemas.Schema), nil
	}
	s, err := schemas.Compile(p.Raw.Content[f.Path])
	if err != nil {
		return nil, err
	}
	e.cache.Store(key, s)
	return s, nil
}

// Evaluate is evaluate(m, g, v) of §4.7 for the PoC roles.
func (e *Evaluator) Evaluate(m domain.Message, g domain.Gate, p *pkgv.Package) domain.EvalResult {
	if p == nil {
		return domain.EvalResult{Findings: []domain.Finding{{Slot: "package", Source: "loader", Severity: "Violation", Message: "package not available"}}}
	}
	res := domain.EvalResult{Pass: true}
	for _, s := range p.Slots {
		if (s.Obligation != "mandatory" && s.Obligation != "recommended") || !contains(s.AppliesTo, string(g)) {
			continue
		}
		var found []domain.Finding
		for _, f := range p.Manifest.Files {
			if f.Slot != s.Slot || !contains(f.AppliesTo, string(g)) {
				continue
			}
			found = append(found, e.apply(m, p, f, s.Slot)...)
		}
		violated := false
		for _, f := range found {
			violated = violated || f.Severity == "Violation"
		}
		if violated && s.Obligation == "mandatory" {
			res.Pass = false
		}
		if violated && s.Obligation == "recommended" {
			res.RecommendedViolations = true
		}
		res.Findings = append(res.Findings, found...)
	}
	return res
}

// apply runs one artefact against the message.
func (e *Evaluator) apply(m domain.Message, p *pkgv.Package, f pkgv.File, slot string) []domain.Finding {
	switch f.Role {
	case "json-schema":
		s, err := e.compiled(p, f)
		if err != nil {
			return []domain.Finding{{Slot: slot, Source: "json-schema", Severity: "Violation", Message: "schema " + f.Path + ": " + err.Error()}}
		}
		var out []domain.Finding
		for _, l := range s.Errors(m.Selected) {
			out = append(out, domain.Finding{Slot: slot, Source: "json-schema", Severity: "Violation", Path: l.Path, Message: l.Message})
		}
		return out
	case "shacl":
		e.warnOnce(p, f)
		return append(e.expansionFindings(m.Root, p, slot), unsupported(slot, f))
	case "context":
		e.warnOnce(p, f)
		return []domain.Finding{unsupported(slot, f)}
	}
	return nil
}

// loader returns the offline loader of a package: only the contexts its manifest declares with url (D-PVP-1).
func (e *Evaluator) loader(p *pkgv.Package) *jsonld.Loader {
	if l, ok := e.loaders.Load(p.Digest); ok {
		return l.(*jsonld.Loader)
	}
	l := jsonld.NewLoader()
	for _, f := range p.Manifest.Files {
		if f.URL != "" {
			_ = l.Add(f.URL, p.Raw.Content[f.Path])
		}
	}
	e.loaders.Store(p.Digest, l)
	return l
}

// expansionFindings expands the whole message as the SHACL gate would; a context outside the package is CONTEXT_NOT_IN_PACKAGE.
func (e *Evaluator) expansionFindings(root []byte, p *pkgv.Package, slot string) []domain.Finding {
	_, err := jsonld.Expand(root, e.loader(p))
	if err == nil {
		return nil
	}
	if url, ok := jsonld.MissingContext(err); ok {
		return []domain.Finding{{Slot: slot, Source: "loader", Severity: "Violation", Message: "@context " + url + " is not in the package", Code: codes.ContextNotInPackage}}
	}
	return []domain.Finding{{Slot: slot, Source: "loader", Severity: "Violation", Message: "JSON-LD expansion failed: " + err.Error()}}
}

// unsupported is the Info finding of a role the PoC does not evaluate; it never changes the classification (DP-7).
func unsupported(slot string, f pkgv.File) domain.Finding {
	return domain.Finding{Slot: slot, Source: "loader", Severity: "Info", Message: string(PocEvalUnsupportedRole) + ": " + f.Role + " artefact " + f.Path + " is not evaluated", Code: PocEvalUnsupportedRole}
}

// warnOnce emits POC_EVAL_UNSUPPORTED_ROLE once per package file.
func (e *Evaluator) warnOnce(p *pkgv.Package, f pkgv.File) {
	if _, loaded := e.warned.LoadOrStore(string(p.Digest)+"|"+f.Path, true); loaded || e.Events == nil {
		return
	}
	e.Events.Emit(app.NewNotice(app.EvSync, e.Namespace, e.Clock.Now(), PocEvalUnsupportedRole, f.Role+" artefact "+f.Path+" is not evaluated by the PoC"))
}

// contains reports whether xs holds x.
func contains(xs []string, x string) bool {
	for _, y := range xs {
		if y == x {
			return true
		}
	}
	return false
}
