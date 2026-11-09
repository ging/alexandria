// F6 replayed with YAML drafts: the hub compiles the language on propose and the client gates with the generated schema.json.
// Also checks that the package carries each slot's artefacts with their gates and that N4 is enforced across releases.
package compiler_dsl_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/server/adapters/compiler_dsl"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// profile is the policy profile IRI every message declares.
const profile = "https://example.com/odrl/profile/f6"

// draft reads a YAML bundle of the F6 scenario, shared with the e2e test.
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
	s := testkit.NewServerWith("2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	s.Publish(t, "2026-09-01T10:00:00Z", draft(t, "v1.0.0.yaml"), app.ApproveOptions{})
	s.Publish(t, "2026-10-01T10:00:00Z", draft(t, "v2.0.0.yaml"), app.ApproveOptions{})
	return s
}

// routes gate the whole Offer and Agreement objects, which the profile schema describes.
func routes() []domain.Route {
	rs := testkit.F6Routes()
	rs[0].Body = &domain.Body{Select: "$.offer"}
	rs[1].Body = &domain.Body{Select: "$.agreement"}
	return rs
}

// request builds a ContractRequestMessage whose profiled offer uses action.
func request(action string) []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld","https://example.com/odrl/profile/f6/context.jsonld"],` +
		`"@type":"ContractRequestMessage","consumerPid":"urn:uuid:c-1","offer":{"@id":"urn:uuid:o-1","@type":"Offer","profile":"` + profile + `",` +
		`"target":"urn:uuid:ds-1","permission":[{"action":"` + action + `"}]}}`)
}

// agreement builds the ContractAgreementMessage of A1.
func agreement() []byte {
	return []byte(`{"@type":"ContractAgreementMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","agreement":{"@id":"urn:uuid:A1",` +
		`"@type":"Agreement","profile":"` + profile + `/1.0.0","target":"urn:uuid:ds-1","permission":[{"action":"ex:access"}]}}`)
}

// at moves the clock and syncs the client.
func at(t *testing.T, s *testkit.Server, c *testkit.Client, ts string) {
	t.Helper()
	s.Clock.Set(ts)
	if err := c.Sync.Run(); err != nil {
		t.Fatalf("sync at %s: %v", ts, err)
	}
}

// send posts a contract request and returns status and verdict header.
func send(c *testkit.Client, action string) (int, string) {
	r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", request(action))
	return r.Status, testkit.VerdictOf(r)
}

// warning is the F6 header of m2 during the grace window.
const warning = `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600`

func TestF6TableWithYAMLDrafts(t *testing.T) {
	s := hub(t)
	c := testkit.NewClient(s, routes())
	steps := []struct {
		at, action string
		status     int
		header     string
	}{
		{"2026-10-02T12:00:00Z", "odrl:use", 200, ""},
		{"2026-10-02T12:00:00Z", "ex:access", 200, ""},
		{"2026-10-02T12:00:00Z", "odrl:read", 400, `violation;gate=policy_intake;code="VERDICT_VIOLATION"`},
		{"2026-10-03T09:59:55Z", "ex:access", 200, warning},
		{"2026-10-03T09:59:55Z", "odrl:read", 200, ""},
		{"2026-10-10T12:00:00Z", "ex:access", 200, warning},
		{"2026-11-05T12:00:00Z", "odrl:use", 200, ""},
		{"2026-11-05T12:00:00Z", "ex:access", 400, `violation;gate=policy_intake;code="VERDICT_VIOLATION"`},
		{"2026-11-05T12:00:00Z", "odrl:read", 200, ""},
	}
	for _, st := range steps {
		at(t, s, c, st.at)
		if status, header := send(c, st.action); status != st.status || header != st.header {
			t.Errorf("%s %s: got %d %q want %d %q", st.at, st.action, status, header, st.status, st.header)
		}
	}
}

func TestAgreementRegisteredWithVersionThatPassed(t *testing.T) {
	s := hub(t)
	c := testkit.NewClient(s, routes())
	at(t, s, c, "2026-10-10T12:00:00Z")
	r := c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", agreement())
	e, ok, _ := c.Repo.Get("urn:uuid:A1")
	if r.Status != http.StatusOK || !ok || e.Version != "1.0.0" {
		t.Fatalf("got %d %+v", r.Status, e)
	}
}

// packageOf returns the manifest of a released version.
func packageOf(t *testing.T, s *testkit.Server, v string) pkgv.Manifest {
	t.Helper()
	b, _, err := s.Hub.Publication.VersionedFile("demo", v, "manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var m pkgv.Manifest
	_ = json.Unmarshal(b, &m)
	return m
}

func TestPackageCarriesSlotArtefactsWithGates(t *testing.T) {
	m := packageOf(t, hub(t), "2.0.0")
	roles := map[string]string{}
	for _, f := range m.Files {
		roles[f.Path] = f.Role + "@" + strings.Join(f.AppliesTo, ",")
	}
	for path, want := range map[string]string{
		"profiles/odrl/f6/2.0.0/schema.json":    "json-schema@policy_intake",
		"profiles/odrl/f6/2.0.0/shapes.ttl":     "shacl@",
		"profiles/odrl/f6/2.0.0/vocab.ttl":      "vocabulary@",
		"profiles/odrl/f6/2.0.0/context.jsonld": "mapping@",
		"space/space-lock.json":                 "",
		compiler_dsl.SourceName:                 "source@",
	} {
		if want == "" {
			continue
		}
		if roles[path] != want {
			t.Errorf("%s: got %q want %q", path, roles[path], want)
		}
	}
	if len(m.Slots) != 1 || m.Slots[0].Slot != "odrl" || m.Slots[0].Obligation != "mandatory" {
		t.Fatalf("slots %+v", m.Slots)
	}
}

// codeOf returns the first code of an application error.
func codeOf(err error) string {
	var f app.Findings
	if errors.As(err, &f) {
		return string(f[0].Code)
	}
	return ""
}

func TestRetiringWithoutDeprecationIsRejected(t *testing.T) {
	s := testkit.NewServerWith("2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	s.Publish(t, "2026-09-01T10:00:00Z", draft(t, "v1.0.0.yaml"), app.ApproveOptions{})
	v2 := string(draft(t, "v2.0.0.yaml"))
	skip := strings.Index(v2, "---\n")
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Editor, []byte(v2[skip+4:]))
	if _, err := s.Hub.Authoring.Propose(testkit.Editor, d.ID); codeOf(err) != "TERM_RETIRED_WITHOUT_DEPRECATION" {
		t.Fatalf("got %v", err)
	}
}

func TestBreakingProfileNeedsMajorSpace(t *testing.T) {
	s := testkit.NewServerWith("2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	s.Publish(t, "2026-09-01T10:00:00Z", draft(t, "v1.0.0.yaml"), app.ApproveOptions{})
	minor := strings.Replace(string(draft(t, "v2.0.0.yaml")), "  version: 2.0.0\n  title: { en: Demo data space }", "  version: 1.1.0\n  title: { en: Demo data space }", 1)
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Editor, []byte(minor))
	if _, err := s.Hub.Authoring.Propose(testkit.Editor, d.ID); codeOf(err) != "VERSION_BUMP_TOO_LOW" {
		t.Fatalf("got %v", err)
	}
}

func TestDraftWithoutSpaceIsRejected(t *testing.T) {
	s := testkit.NewServerWith("2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	v1 := string(draft(t, "v1.0.0.yaml"))
	_, err := s.Hub.Authoring.CreateDraft(testkit.Editor, []byte(v1[:strings.Index(v1, "---")]))
	if codeOf(err) != string(compiler_dsl.CodeBundleWithoutSpace) {
		t.Fatalf("got %v", err)
	}
}
