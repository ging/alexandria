// Tests of the PoC evaluator's JSON-LD loader check, which expands the whole message with json-gold offline.
// A context nested inside the message, not only at the top, must be declared in the manifest (D-PVP-1, REQ-CLI-010).
package evaluator_poc

import (
	"testing"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/pkgv"
)

// dspContext is the context URL the package declares.
const dspContext = "https://w3id.org/dspace/2025/1/context.jsonld"

// shaclPackage is a package whose odrl slot has a SHACL artefact and declares one context.
func shaclPackage() *pkgv.Package {
	return &pkgv.Package{
		Digest: "ni:///sha-256;test",
		Manifest: pkgv.Manifest{Slots: []pkgv.Slot{{Slot: "odrl", Obligation: "mandatory"}}, Files: []pkgv.File{
			{Path: "shapes.ttl", Role: "shacl", Slot: "odrl", AppliesTo: []string{"policy_intake"}},
			{Path: "ctx.jsonld", Role: "mapping", Slot: "odrl", URL: dspContext, AppliesTo: []string{}},
		}},
		Raw: pkgv.Files{Content: map[string][]byte{"shapes.ttl": []byte("."), "ctx.jsonld": []byte(`{"@context":{"offer":"https://w3id.org/dspace/2025/1/offer"}}`)}},
	}
}

// evaluate runs the policy_intake gate on a whole message.
func evaluate(msg string) domain.EvalResult {
	e := &Evaluator{}
	return e.Evaluate(domain.Message{Root: []byte(msg), Selected: []byte(msg)}, domain.GatePolicy, shaclPackage())
}

func TestDeclaredContextPasses(t *testing.T) {
	if r := evaluate(`{"@context":"` + dspContext + `","@id":"urn:m"}`); !r.Pass {
		t.Fatalf("got %+v", r.Findings)
	}
}

func TestNestedUndeclaredContextIsCaught(t *testing.T) {
	r := evaluate(`{"@context":"` + dspContext + `","@id":"urn:m","offer":{"@context":"https://evil.example/ctx.jsonld","@id":"urn:o"}}`)
	if r.Pass || len(r.Findings) != 1 || r.Findings[0].Code != codes.ContextNotInPackage {
		t.Fatalf("got %+v", r)
	}
}
