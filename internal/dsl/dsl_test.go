// Tests of the profile language: the spec examples compile, each validation level reports its code, outputs are deterministic.
// Policies of §6.6 and §33 are checked against the generated schema.json, the artefact the vhub gates run.
package dsl

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/semver"
)

// colors returns the §29 profile, optionally with replacements applied in order (old, new, old, new...).
func colors(t *testing.T, repl ...string) Source {
	t.Helper()
	b, err := os.ReadFile("testdata/colors.profile.yaml")
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	for i := 0; i+1 < len(repl); i += 2 {
		if !strings.Contains(s, repl[i]) {
			t.Fatalf("fixture lacks %q", repl[i])
		}
		s = strings.Replace(s, repl[i], repl[i+1], 1)
	}
	return Source{Name: "colors.profile.yaml", Data: []byte(s)}
}

// space builds a SpaceProfile binding the colors profile with the given version range and extra bindings.
func space(version, rng, extra string) Source {
	return Source{Name: "demo.space.yaml", Data: []byte(`dsl: alexandria.profiles/v1
kind: SpaceProfile
space:
  id: https://example.com/spaces/demo/profile
  version: ` + version + `
  title: { en: Demo }
  operator: https://example.com/party/demo-authority
  base_url: https://example.com/spaces/demo
bindings:
  - slot: odrl
    profile: https://example.com/odrl/profile/colors
    source: internal
    version: "` + rng + `"
    obligation: mandatory
    conformance:
      - { role: shacl, applies_to: policy_intake }
      - { role: json-schema, applies_to: dsp_message }
` + extra)}
}

// withCedar enables the disconnected Cedar projection for one test.
func withCedar(t *testing.T) {
	t.Helper()
	cedarEnabled = true
	t.Cleanup(func() { cedarEnabled = false })
}

// codes returns the codes of the findings with a severity.
func codes(r *Result, sev string) []string {
	var out []string
	for _, f := range r.Findings {
		if f.Severity == sev {
			out = append(out, f.Code)
		}
	}
	return out
}

// expectErrors checks that the errors are exactly the given codes.
func expectErrors(t *testing.T, r *Result, want ...string) {
	t.Helper()
	got := codes(r, Error)
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("errors %v, want %v\n%v", got, want, r.Findings)
	}
}

// artifact returns the bytes of an output path.
func artifact(t *testing.T, r *Result, path string) []byte {
	t.Helper()
	for _, a := range r.Artifacts {
		if a.Path == path {
			return a.Bytes
		}
	}
	t.Fatalf("no artefact %s", path)
	return nil
}

func TestSpecProfileExampleCompiles(t *testing.T) {
	r := Compile(Options{}, colors(t))
	expectErrors(t, r)
	if len(r.Artifacts) != 6 {
		t.Fatalf("%d artefacts, want 6", len(r.Artifacts))
	}
}

func TestCedarIsDisconnectedByDefault(t *testing.T) {
	r := Compile(Options{}, colors(t))
	for _, a := range r.Artifacts {
		if strings.Contains(a.Path, "cedar") || strings.Contains(a.Role, "cedar") {
			t.Fatalf("unexpected Cedar artefact %s (%s)", a.Path, a.Role)
		}
	}
	if vocab := string(artifact(t, r, "profiles/odrl/colors/0.1.0/vocab.ttl")); strings.Contains(vocab, "cedarPath") {
		t.Fatal("vocab.ttl carries alx:cedarPath while Cedar is disconnected")
	}
}

func TestCedarRoleIsUnavailableWhileDisconnected(t *testing.T) {
	sp := space("1.0.0", "^0.1", "")
	sp.Data = bytes.Replace(sp.Data, []byte("      - { role: json-schema"), []byte("      - { role: cedar-mapping, applies_to: pdp_load }\n      - { role: json-schema"), 1)
	expectErrors(t, Compile(Options{}, colors(t), sp), CodeArtifactMissing)
}

func TestCedarProjectionWhenEnabled(t *testing.T) {
	withCedar(t)
	r := Compile(Options{}, colors(t))
	expectErrors(t, r)
	if len(r.Artifacts) != 8 {
		t.Fatalf("%d artefacts, want 8", len(r.Artifacts))
	}
	if vocab := string(artifact(t, r, "profiles/odrl/colors/0.1.0/vocab.ttl")); !strings.Contains(vocab, "alx:cedarPath") {
		t.Fatal("vocab.ttl lacks alx:cedarPath with Cedar enabled")
	}
}

func TestSpecSpaceExampleNeedsVendoredCopiesAndHasAnImpossibleBinding(t *testing.T) {
	sp, _ := os.ReadFile("testdata/demo.space.yaml")
	r := Compile(Options{}, colors(t), Source{Name: "demo.space.yaml", Data: sp})
	expectErrors(t, r, CodeBindingUnresolved, CodeBindingUnresolved, CodeBindingBaseMismatch)
	if !strings.Contains(strings.Join(codes(r, Warning), ","), CodeComplianceIgnored) {
		t.Fatal("conformance_required should be reported as ignored")
	}
}

func TestShapesFollowReferenceOutput(t *testing.T) {
	sh := string(artifact(t, Compile(Options{}, colors(t)), "profiles/odrl/colors/0.1.0/shapes.ttl"))
	for _, want := range []string{
		"sh:in ( col:putColor odrl:use odrl:transfer )",
		"sh:in ( odrl:dateTime col:color )",
		"sh:in ( odrl:eq odrl:lt odrl:lteq odrl:gt odrl:gteq )",
		"sh:nodeKind sh:Literal ; sh:datatype xsd:dateTime",
		"sh:nodeKind sh:IRI ; sh:in ( col:red col:green col:blue col:yellow )",
		"sh:path odrl:rightOperandReference ; sh:maxCount 0",
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("shapes.ttl lacks %q", want)
		}
	}
}

func TestContextHasOneAliasPerTerm(t *testing.T) {
	ctx := string(artifact(t, Compile(Options{}, colors(t)), "profiles/odrl/colors/0.1.0/context.jsonld"))
	for _, want := range []string{`"putColor":"col:putColor"`, `"color":"col:color"`, `"red":"col:red"`, `"dateTime":"http://www.w3.org/ns/odrl/2/dateTime"`, `"col":"https://example.com/ns/colors#"`} {
		if !strings.Contains(ctx, want) {
			t.Errorf("context lacks %s: %s", want, ctx)
		}
	}
}

func TestCompilationIsDeterministic(t *testing.T) {
	a, b := Compile(Options{}, colors(t), space("1.0.0", "^0.1", "")), Compile(Options{}, colors(t), space("1.0.0", "^0.1", ""))
	if len(a.Artifacts) == 0 || len(a.Artifacts) != len(b.Artifacts) {
		t.Fatalf("artefacts: %d vs %d %v", len(a.Artifacts), len(b.Artifacts), a.Findings)
	}
	for i := range a.Artifacts {
		if !bytes.Equal(a.Artifacts[i].Bytes, b.Artifacts[i].Bytes) {
			t.Errorf("%s differs between runs", a.Artifacts[i].Path)
		}
	}
	if !bytes.Equal(a.Lock.JSON(), b.Lock.JSON()) {
		t.Error("space-lock.json differs between runs")
	}
}

func TestN1RejectsAnchorsAliasesAndTags(t *testing.T) {
	for name, doc := range map[string]string{
		"anchor": "dsl: &x alexandria.profiles/v1\nkind: OdrlProfile\n",
		"alias":  "a: &x 1\nb: *x\n",
		"tag":    "dsl: !custom alexandria.profiles/v1\nkind: OdrlProfile\n",
	} {
		r := Compile(Options{}, Source{Name: name, Data: []byte(doc)})
		expectErrors(t, r, CodeYAML)
	}
}

func TestN1RejectsUnknownField(t *testing.T) {
	r := Compile(Options{}, colors(t, "rules:", "colour: red\nrules:"))
	if got := codes(r, Error); len(got) == 0 || got[0] != CodeSchema || r.Findings[0].Line == 0 {
		t.Fatalf("got %v", r.Findings)
	}
}

func TestN2OrderedOperatorNeedsOrderedType(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "operators: [eq, neq]", "operators: [eq, lt]")), CodeSigOperatorType)
}

func TestN2UnknownProfileOperator(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "operators: [eq, neq]", "operators: [eq, col:doubleOf]")), CodeSigOperatorUnknown)
}

func TestN2DuplicateTermID(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "  - id: green\n", "  - id: red\n")), CodeTermDupID)
}

func TestN2EnforcementRequiresSignature(t *testing.T) {
	r := Compile(Options{}, colors(t, "    signature:\n      value_kind: iri\n      values: [col:red, col:green, col:blue, col:yellow]\n      operators: [eq, neq]\n", ""))
	expectErrors(t, r, CodeEnforcementNoSig)
}

func TestN2UndeclaredPrefix(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "included_in: odrl:use", "included_in: foo:use")), CodePrefixUndeclared)
}

func TestN2MissingIncludedInIsWarning(t *testing.T) {
	r := Compile(Options{}, colors(t, "    included_in: odrl:use\n", ""))
	expectErrors(t, r)
	if strings.Join(codes(r, Warning), ",") != CodeIncludedInMissing {
		t.Fatalf("warnings %v", codes(r, Warning))
	}
}

func TestN2SlotDuplicated(t *testing.T) {
	dup := "  - slot: odrl\n    profile: https://example.com/odrl/profile/colors\n    source: internal\n    version: \"^0.1\"\n    obligation: optional\n"
	expectErrors(t, Compile(Options{}, colors(t), space("1.0.0", "^0.1", dup)), CodeSlotDup)
}

func TestN2LifecycleInvariants(t *testing.T) {
	sp := space("1.0.0", "^0.1", "")
	sp.Data = bytes.Replace(sp.Data, []byte("bindings:"), []byte("lifecycle:\n  clock_skew: PT10S\n  lease_margin: PT5S\nbindings:"), 1)
	expectErrors(t, Compile(Options{}, colors(t), sp), "LIFECYCLE_LEASE")
}

func TestN3AdoptedKindMismatch(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "    - term: odrl:dateTime", "    - term: odrl:read")), CodeAdoptedKindMismatch)
}

func TestN3AdoptedUnknown(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t, "    - term: odrl:dateTime", "    - term: odrl:access")), CodeAdoptedUnknown)
}

func TestN3BindingUnresolvedByRange(t *testing.T) {
	expectErrors(t, Compile(Options{}, colors(t), space("1.0.0", "^1.0", "")), CodeBindingUnresolved)
}

func TestN3ArtifactMissingWithoutEnforcement(t *testing.T) {
	noEnf := colors(t, "    enforcement:\n      cedar_path: context.color\n      pdp_level: PDP-4\n      eval_timing: once\n", "")
	sp := space("1.0.0", "^0.1", "")
	sp.Data = bytes.Replace(sp.Data, []byte("      - { role: json-schema"), []byte("      - { role: cedar-mapping, applies_to: pdp_load }\n      - { role: json-schema"), 1)
	expectErrors(t, Compile(Options{}, noEnf, sp), CodeArtifactMissing)
}

func TestN3VendoredBindingResolvesLocally(t *testing.T) {
	vendor := FSVendor{FS: fstest.MapFS{
		"dsp/2025-1/vendor.json":       {Data: []byte(`{"profile":"https://w3id.org/dspace/2025/1","version":"2025-1","artifacts":[{"path":"offer.schema.json","role":"json-schema","media_type":"application/schema+json"}]}`)},
		"dsp/2025-1/offer.schema.json": {Data: []byte(`{"type":"object"}`)},
	}}
	dsp := "  - slot: dsp\n    profile: https://w3id.org/dspace/2025/1\n    source: vendored\n    version: \"=2025-1\"\n    obligation: mandatory\n    conformance:\n      - { role: json-schema, applies_to: dsp_message }\n"
	r := Compile(Options{Vendor: vendor}, colors(t), space("1.0.0", "^0.1", dsp))
	expectErrors(t, r)
	artifact(t, r, "profiles/dsp/vendored/2025-1/offer.schema.json")
}

// release returns a variant of the colors profile at version v with extra replacements.
func release(t *testing.T, v string, repl ...string) Source {
	return colors(t, append([]string{"version: 0.1.0", "version: " + v}, repl...)...)
}

// deprecate marks the red right operand with a status.
func deprecate(status string) []string {
	return []string{"    definition: { en: The colour red (#FF0000). }\n", "    definition: { en: The colour red (#FF0000). }\n    status: " + status + "\n    replaced_by: col:green\n"}
}

func TestN4DeprecationIsMinor(t *testing.T) {
	expectErrors(t, Compile(Options{Previous: []Source{colors(t)}}, release(t, "0.2.0", deprecate("deprecated")...)))
}

func TestN4RetireWithoutDeprecation(t *testing.T) {
	expectErrors(t, Compile(Options{Previous: []Source{colors(t)}}, release(t, "1.0.0", deprecate("retired")...)), CodeRetiredNoDeprecation)
}

func TestN4RetireAfterDeprecationNeedsMajor(t *testing.T) {
	prev := release(t, "0.2.0", deprecate("deprecated")...)
	expectErrors(t, Compile(Options{Previous: []Source{prev}}, release(t, "0.3.0", deprecate("retired")...)), CodeVersionBumpTooLow)
	expectErrors(t, Compile(Options{Previous: []Source{prev}}, release(t, "1.0.0", deprecate("retired")...)))
}

func TestN4RestrictedSignatureNeedsMajor(t *testing.T) {
	r := Compile(Options{Previous: []Source{colors(t)}}, release(t, "0.2.0", "operators: [eq, neq]", "operators: [eq]"))
	expectErrors(t, r, CodeVersionBumpTooLow)
}

func TestN4TermRemoved(t *testing.T) {
	gone := release(t, "1.0.0", "  - id: yellow\n    kind: right_operand\n    label: { en: Yellow }\n    definition: { en: The colour yellow (#FFFF00). }\n", "")
	expectErrors(t, Compile(Options{Previous: []Source{colors(t)}}, gone), CodeTermRemoved)
}

func TestN4ReleasedVersionCannotChange(t *testing.T) {
	expectErrors(t, Compile(Options{Previous: []Source{colors(t)}}, colors(t, "Put color", "Put colour")), CodeProfileVersionChanged)
}

func TestN4SpaceMandatoryBindingNeedsMajor(t *testing.T) {
	dcp := "  - slot: dcat\n    profile: https://example.com/dcat\n    source: vendored\n    version: \"=1\"\n    obligation: mandatory\n    conformance:\n      - { role: shacl, applies_to: catalog_entry }\n"
	vendor := FSVendor{FS: fstest.MapFS{
		"dcat/1/vendor.json": {Data: []byte(`{"profile":"https://example.com/dcat","version":"1","artifacts":[{"path":"s.ttl","role":"shacl","media_type":"text/turtle"}]}`)},
		"dcat/1/s.ttl":       {Data: []byte(`<a> <b> <c> .`)},
	}}
	prev := []Source{colors(t), space("1.0.0", "^0.1", "")}
	expectErrors(t, Compile(Options{Previous: prev, Vendor: vendor}, colors(t), space("1.1.0", "^0.1", dcp)), CodeVersionBumpTooLow)
	expectErrors(t, Compile(Options{Previous: prev, Vendor: vendor}, colors(t), space("2.0.0", "^0.1", dcp)))
}

func TestN5TurtleCheck(t *testing.T) {
	if checkTurtle([]byte("<a> <b> ( <c> .")) == nil || checkTurtle([]byte(`<a> <b> "x .`)) == nil || checkTurtle([]byte("<a> <b> <c> .")) != nil {
		t.Fatal("turtle check")
	}
}

func TestSatisfiesRanges(t *testing.T) {
	cases := map[string]map[string]bool{
		"^0.1":   {"0.1.0": true, "0.1.9": true, "0.2.0": false},
		"^1.2":   {"1.2.0": true, "1.9.0": true, "2.0.0": false, "1.1.9": false},
		"~1.4":   {"1.4.3": true, "1.5.0": false},
		"=1.2.0": {"1.2.0": true, "1.2.1": false},
		"1.2":    {"1.2.7": true, "1.3.0": false},
	}
	for rng, vs := range cases {
		for v, want := range vs {
			if got := Satisfies(rng, mustSemver(v)); got != want {
				t.Errorf("%s %s: got %v", rng, v, got)
			}
		}
	}
}

// offer wraps a permission in a profiled Offer in DSP compact form.
func offer(profile, permission string) []byte {
	p := ""
	if profile != "" {
		p = `"profile":"` + profile + `",`
	}
	return []byte(`{"@type":"Offer","@id":"urn:uuid:o",` + p + `"permission":[` + permission + `]}`)
}

// interval is the C1 constraint of the motivation: dateTime in [2025, 2026).
const interval = `{"and":[{"leftOperand":"dateTime","operator":"gteq","rightOperand":{"@value":"2025-01-01T00:00:00Z","@type":"xsd:dateTime"}},` +
	`{"leftOperand":"dateTime","operator":"lt","rightOperand":{"@value":"2026-01-01T00:00:00Z","@type":"xsd:dateTime"}}]}`

func TestGeneratedSchemaReproducesSection33(t *testing.T) {
	s, err := schemas.Compile(artifact(t, Compile(Options{}, colors(t)), "profiles/odrl/colors/0.1.0/schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	id := "https://example.com/odrl/profile/colors"
	cases := []struct {
		name  string
		doc   []byte
		valid bool
	}{
		{"6.6 valid", offer(id, `{"action":"putColor","constraint":[{"leftOperand":"color","operator":"eq","rightOperand":{"@id":"col:red"}}]}`), true},
		{"6.6 invalid gt and literal", offer(id, `{"action":"use","constraint":[{"leftOperand":"color","operator":"gt","rightOperand":"#FF0000"}]}`), false},
		{"6.6 invalid print", offer(id, `{"action":"odrl:print"}`), false},
		{"C1 use + interval", offer(id, `{"action":"use","constraint":[`+interval+`]}`), true},
		{"C2 access + interval", offer(id, `{"action":"access","constraint":[`+interval+`]}`), false},
		{"putColor + dateTime + color", offer(id, `{"action":"putColor","constraint":[`+interval+`,{"leftOperand":"color","operator":"eq","rightOperand":{"@id":"col:red"}}]}`), true},
		{"C1 untyped date", offer(id, `{"action":"use","constraint":[{"leftOperand":"dateTime","operator":"gteq","rightOperand":"2025-01-01"}]}`), false},
		{"no profile declared", offer("", `{"action":"putColor"}`), false},
		{"versioned profile IRI", offer(id+"/0.1.0", `{"action":"putColor"}`), true},
	}
	for _, c := range cases {
		if err := s.Validate(c.doc); (err == nil) != c.valid {
			t.Errorf("%s: valid=%v, err=%v", c.name, err == nil, err)
		}
	}
}

func TestSpaceArtefactsCarryGates(t *testing.T) {
	r := Compile(Options{}, colors(t), space("1.0.0", "^0.1", ""))
	expectErrors(t, r)
	gates := map[string]string{}
	for _, a := range r.Artifacts {
		gates[a.Path] = strings.Join(a.AppliesTo, ",")
	}
	if gates["profiles/odrl/colors/0.1.0/shapes.ttl"] != "policy_intake" || gates["profiles/odrl/colors/0.1.0/schema.json"] != "dsp_message" {
		t.Fatalf("gates %v", gates)
	}
	lock := string(r.Lock.JSON())
	if !strings.Contains(lock, `"version":"0.1.0"`) || !strings.Contains(lock, `"obligation":"mandatory"`) || !strings.Contains(lock, `"sha256":`) {
		t.Fatalf("lock %s", lock)
	}
	artifact(t, r, "space/shacl-union.ttl")
}

// mustSemver parses a version for range tests.
func mustSemver(s string) semver.Version { return semver.MustParse(s) }

func TestClosedListWithoutLeftOperandsForbidsConstraints(t *testing.T) {
	src := Source{Name: "p.yaml", Data: []byte("dsl: alexandria.profiles/v1\nkind: OdrlProfile\nprofile:\n  id: https://example.com/p\n  version: 1.0.0\n  title: { en: P }\n  publisher: X\n  license: https://example.com/l\n  base_url: https://example.com/p\nnamespaces:\n  p: https://example.com/ns/p#\n")}
	r := Compile(Options{}, src)
	expectErrors(t, r)
	s, err := schemas.Compile(artifact(t, r, "profiles/odrl/p/1.0.0/schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	if s.Validate(offer("https://example.com/p", `{"action":"use","constraint":[{"leftOperand":"dateTime","operator":"eq","rightOperand":"x"}]}`)) == nil {
		t.Fatal("a constraint passed a profile without left operands")
	}
}

func TestN5ContextAliasesExpandToTermIRIs(t *testing.T) {
	r := Compile(Options{}, colors(t))
	expectErrors(t, r)
	ix := r.Profiles[0]
	if err := checkContext(ix, artifact(t, r, "profiles/odrl/colors/0.1.0/context.jsonld")); err != nil {
		t.Fatal(err)
	}
	broken := []byte(`{"@context":{"@version":1.1,"col":"https://example.com/ns/other#","putColor":"col:putColor"}}`)
	if err := checkContext(ix, broken); err == nil {
		t.Fatal("a context that maps aliases to other IRIs passed")
	}
}
