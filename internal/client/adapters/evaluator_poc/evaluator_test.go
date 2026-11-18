// Tests of the PoC evaluator's JSON-LD loader check, which expands the whole message with json-gold offline.
// A context nested inside the message, not only at the top, must be declared in the manifest (D-PVP-1, REQ-CLI-010).
package evaluator_poc

import (
	"testing"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// sink records the events an evaluator emits.
type sink struct{ events []any }

// Emit stores one event.
func (s *sink) Emit(e any) { s.events = append(s.events, e) }

// fixedClock answers always the same instant.
type fixedClock struct{}

// Now returns the fixed instant.
func (fixedClock) Now() timex.Instant { return timex.MustInstant("2026-09-03T12:00:00Z") }

// dspContext is the context URL the package declares.
const dspContext = "https://w3id.org/dspace/2025/1/context.jsonld"

// shaclPackage is a package whose odrl slot has a SHACL artefact and declares one context.
func shaclPackage() *pkgv.Package {
	return &pkgv.Package{
		Digest: "ni:///sha-256;test",
		Slots:  []pkgv.Slot{{Slot: "odrl", Obligation: "mandatory", AppliesTo: []string{"policy_intake"}}},
		Manifest: pkgv.Manifest{Files: []pkgv.File{
			{Path: "shapes.ttl", Role: "shacl", Slot: "odrl", AppliesTo: []string{"policy_intake"}},
			{Path: "ctx.jsonld", Role: "other", Slot: "odrl", URL: dspContext},
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
	if r.Pass || len(r.Findings) != 2 || r.Findings[0].Code != codes.ContextNotInPackage {
		t.Fatalf("got %+v", r)
	}
}

func TestShaclIsReportedAsUnsupportedInfo(t *testing.T) {
	r := evaluate(`{"@context":"` + dspContext + `","@id":"urn:m"}`)
	if !r.Pass || len(r.Findings) != 1 || r.Findings[0].Severity != "Info" || r.Findings[0].Code != PocEvalUnsupportedRole {
		t.Fatalf("got %+v", r)
	}
}

func TestSlotOfAnotherGateIsNotEvaluated(t *testing.T) {
	e := &Evaluator{}
	if r := e.Evaluate(domain.Message{Root: []byte(`{}`), Selected: []byte(`{}`)}, domain.GateDSP, shaclPackage()); !r.Pass || len(r.Findings) != 0 {
		t.Fatalf("got %+v", r)
	}
}

func TestUnsupportedRoleNoticeIsInformational(t *testing.T) {
	out := &sink{}
	e := &Evaluator{Events: out, Clock: fixedClock{}, Namespace: "ns"}
	msg := []byte(`{"@context":"` + dspContext + `","@id":"urn:m"}`)
	e.Evaluate(domain.Message{Root: msg, Selected: msg}, domain.GatePolicy, shaclPackage())
	if len(out.events) != 1 {
		t.Fatalf("%d events, want 1", len(out.events))
	}
	if n, ok := out.events[0].(app.Notice); !ok || n.Severity != string(codes.SevInfo) || n.Code != string(PocEvalUnsupportedRole) {
		t.Fatalf("event %+v", out.events[0])
	}
}

// TestJSONSchemaTargetsRootForDSPAndSelectedForPolicy verifies gate-scoped schema evaluation on nested DSP messages.
func TestJSONSchemaTargetsRootForDSPAndSelectedForPolicy(t *testing.T) {
	pkg := &pkgv.Package{
		Digest: "ni:///sha-256;dual",
		Slots: []pkgv.Slot{
			{Slot: "dsp", Obligation: "mandatory", AppliesTo: []string{"dsp_message"}},
			{Slot: "odrl", Obligation: "mandatory", AppliesTo: []string{"policy_intake"}},
		},
		Manifest: pkgv.Manifest{Files: []pkgv.File{
			{Path: "dsp.json", Role: "json-schema", Slot: "dsp", AppliesTo: []string{"dsp_message"}},
			{Path: "odrl.json", Role: "json-schema", Slot: "odrl", AppliesTo: []string{"policy_intake"}},
		}},
		Raw: pkgv.Files{Content: map[string][]byte{
			"dsp.json":  []byte(`{"type":"object","required":["@type","offer"]}`),
			"odrl.json": []byte(`{"type":"object","required":["profile"]}`),
		}},
	}
	msg := domain.Message{
		Root:     []byte(`{"@type":"ContractRequestMessage","offer":{"profile":"https://example.org/odrl"}}`),
		Selected: []byte(`{"profile":"https://example.org/odrl"}`),
	}
	ev := &Evaluator{}
	if r := ev.Evaluate(msg, domain.GateDSP, pkg); !r.Pass {
		t.Fatalf("dsp_message failed: %+v", r.Findings)
	}
	if r := ev.Evaluate(msg, domain.GatePolicy, pkg); !r.Pass {
		t.Fatalf("policy_intake failed: %+v", r.Findings)
	}
}
