// PoC evaluator: runs json-schema artefacts offline and only checks @context URLs against the manifest for SHACL slots.
// SHACL and JSON-LD artefacts are ignored with a POC_EVAL_UNSUPPORTED_ROLE info event; the §4.7 slot algorithm is kept (DP-7, REQ-CLI-010).
package evaluator_poc

import (
	"encoding/json"
	"sync"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
)

// Evaluator compiles package schemas once and validates bodies without network.
type Evaluator struct {
	Events app.EventSink
	Clock  timex.Clock
	Space  string
	cache  sync.Map
	warned sync.Map
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
func (e *Evaluator) Evaluate(body []byte, g domain.Gate, p *pkgv.Package) domain.EvalResult {
	if p == nil {
		return domain.EvalResult{Findings: []domain.Finding{{Slot: "odrl", Source: "loader", Severity: "Violation", Message: "package not available"}}}
	}
	res := domain.EvalResult{Pass: true}
	for _, s := range p.Manifest.Slots {
		if s.Obligation != "mandatory" && s.Obligation != "recommended" {
			continue
		}
		var found []domain.Finding
		for _, f := range p.Manifest.Files {
			if f.Slot != s.Slot || !contains(f.AppliesTo, string(g)) {
				continue
			}
			found = append(found, e.apply(body, p, f, s.Slot)...)
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

// apply runs one artefact against the body.
func (e *Evaluator) apply(body []byte, p *pkgv.Package, f pkgv.File, slot string) []domain.Finding {
	switch f.Role {
	case "json-schema":
		s, err := e.compiled(p, f)
		if err != nil {
			return []domain.Finding{{Slot: slot, Source: "json-schema", Severity: "Violation", Message: "schema " + f.Path + ": " + err.Error()}}
		}
		var out []domain.Finding
		for _, l := range s.Errors(body) {
			out = append(out, domain.Finding{Slot: slot, Source: "json-schema", Severity: "Violation", Path: l.Path, Message: l.Message})
		}
		return out
	case "shacl":
		e.warnOnce(p, f)
		return contextFindings(body, p.Manifest.URLs(), slot)
	}
	e.warnOnce(p, f)
	return nil
}

// contextFindings reports every @context URL not declared in the manifest (D-PVP-1), without any network access.
func contextFindings(body []byte, allowed map[string]bool, slot string) []domain.Finding {
	var obj map[string]any
	if json.Unmarshal(body, &obj) != nil {
		return nil
	}
	var urls []string
	switch c := obj["@context"].(type) {
	case string:
		urls = append(urls, c)
	case []any:
		for _, x := range c {
			if s, ok := x.(string); ok {
				urls = append(urls, s)
			}
		}
	}
	var out []domain.Finding
	for _, u := range urls {
		if !allowed[u] {
			out = append(out, domain.Finding{Slot: slot, Source: "loader", Severity: "Violation", Message: "@context " + u + " is not in the package", Code: codes.ContextNotInPackage})
		}
	}
	return out
}

// warnOnce emits POC_EVAL_UNSUPPORTED_ROLE once per package file.
func (e *Evaluator) warnOnce(p *pkgv.Package, f pkgv.File) {
	if _, loaded := e.warned.LoadOrStore(string(p.Digest)+"|"+f.Path, true); loaded || e.Events == nil {
		return
	}
	e.Events.Emit(app.NewNotice(app.EvSync, e.Space, e.Clock.Now(), PocEvalUnsupportedRole, f.Role+" artefact "+f.Path+" is not evaluated by the PoC"))
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
