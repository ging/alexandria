// Tests the language compiler adapter on the F6 YAML drafts: both releases publish, manifests validate and N4 is enforced.
// Fixtures: conformance/f6/drafts and a testkit hub using compiler_dsl (REQ-HUB-001, DP-6).
package compiler_dsl_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/adapters/compiler_dsl"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// draft reads a YAML bundle of the F6 scenario.
func draft(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../../conformance/f6/drafts/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// hub publishes the YAML F6 releases at their F6 instants.
func hub(t *testing.T) *testkit.Server {
	t.Helper()
	s := testkit.NewServerWith(t, "2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	s.Publish(t, "2026-09-01T10:00:00Z", draft(t, "v1.0.0.yaml"), app.ApproveOptions{})
	s.Publish(t, "2026-10-01T10:00:00Z", draft(t, "v2.0.0.yaml"), app.ApproveOptions{})
	return s
}

// manifest returns the signed manifest of a version.
func manifest(t *testing.T, s *testkit.Server, v string) []byte {
	t.Helper()
	raw, err := s.Hub.Listing.Record(testkit.NS, v)
	if err != nil {
		t.Fatal(err)
	}
	var rec vc.Credential[lifecycle.Record]
	_ = json.Unmarshal(raw, &rec)
	m, err := s.Hub.Publication.Manifest(rec.Subject.Manifest.Segment())
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestYAMLReleasesHaveBreakingClass(t *testing.T) {
	s := hub(t)
	raw, _ := s.Hub.Listing.Record(testkit.NS, "2.0.0")
	var rec vc.Credential[lifecycle.Record]
	_ = json.Unmarshal(raw, &rec)
	if rec.Subject.Class != lifecycle.Breaking {
		t.Fatalf("class %s", rec.Subject.Class)
	}
}

func TestManifestValidatesAndCarriesSlots(t *testing.T) {
	m := manifest(t, hub(t), "1.0.0")
	if err := schemas.Validate(schemas.Manifest, m); err != nil {
		t.Fatal(err)
	}
	c, _ := pkgv.Parse(m)
	found := false
	for _, f := range c.Subject.Files {
		found = found || f.Path == pkgv.SlotsPath
	}
	if !found {
		t.Fatal("profile/slots.json missing")
	}
}

func TestDraftWithoutSpaceIsRejected(t *testing.T) {
	s := testkit.NewServerWith(t, "2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	_, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, []byte("kind: OdrlProfile\n"))
	var f app.Findings
	if !errors.As(err, &f) || len(f) == 0 {
		t.Fatalf("got %v", err)
	}
}

func TestSlotsListEachGateOnce(t *testing.T) {
	doc := bytes.Replace(draft(t, "v1.0.0.yaml"), []byte("      - { role: json-schema, applies_to: policy_intake }"),
		[]byte("      - { role: json-schema, applies_to: policy_intake }\n      - { role: shacl, applies_to: policy_intake }"), 1)
	c, err := compiler_dsl.Compiler{}.Compile(doc, app.CompileContext{})
	if err != nil {
		t.Fatal(err)
	}
	var slots []pkgv.Slot
	if err := json.Unmarshal(c.Files[pkgv.SlotsPath], &slots); err != nil || len(slots) != 1 {
		t.Fatalf("slots %s: %v", c.Files[pkgv.SlotsPath], err)
	}
	if got := slots[0].AppliesTo; len(got) != 1 || got[0] != "policy_intake" {
		t.Fatalf("applies_to %v", got)
	}
}

// TestCompilerAppliesServerLifecycleDefaults verifies that server lifecycle defaults fill an absent space lifecycle.
func TestCompilerAppliesServerLifecycleDefaults(t *testing.T) {
	defs := lifecycle.Defaults()
	defs.GraceWindow *= 2
	c, err := compiler_dsl.Compiler{Defaults: defs}.Compile(draft(t, "v1.0.0.yaml"), app.CompileContext{})
	if err != nil {
		t.Fatal(err)
	}
	if c.Params.GraceWindow != defs.GraceWindow {
		t.Fatalf("got grace_window %v, want %v", c.Params.GraceWindow, defs.GraceWindow)
	}
}

// TestValidateReportsSuggestedBumpOnStaleDraft verifies Authoring.Validate exposes suggested_version and required_bump.
func TestValidateReportsSuggestedBumpOnStaleDraft(t *testing.T) {
	s := testkit.NewServerWith(t, "2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	v1 := draft(t, "v1.0.0.yaml")
	s.Publish(t, "2026-09-01T10:00:00Z", v1, app.ApproveOptions{})
	edited := bytes.Replace(v1, []byte("definition: { en: To access the asset. }"), []byte("definition: { en: To access the asset. }\n  - id: readExtra\n    kind: action\n    included_in: odrl:use\n    label: { en: Read Extra }\n    definition: { en: Extra action. }"), 1)
	d, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, edited)
	if err != nil {
		t.Fatal(err)
	}
	rep, err := s.Hub.Authoring.Validate(testkit.Sergio, testkit.NS, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.SuggestedVersion != "1.1.0" || rep.RequiredBump != "MINOR" {
		t.Fatalf("unexpected report: %+v", rep)
	}
}
