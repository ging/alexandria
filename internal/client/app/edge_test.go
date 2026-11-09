// Tests of the hot path and the state machine: classification, actions, responses, agreements, audit, events and status.
// Realise T-CLI-002, 008, 010, 012, 013, 015..018, 020..023, 026 and T-SYS-001, 002, 004, 007, 008 (§4.5–§4.11).
package app_test

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	clientapp "github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// request sends a ContractRequestMessage through the F6 route.
func request(c *testkit.Client, action string) *clientapp.Response {
	return c.Send(domain.DSPIngress, "POST", "/negotiations/request", testkit.ContractRequest(action))
}

// withMode returns the F6 routes with every mode replaced.
func withMode(m domain.Mode) []domain.Route {
	rs := testkit.F6Routes()
	for i := range rs {
		rs[i].Mode = m
	}
	return rs
}

// catalogDraft builds a draft whose dcat slot validates catalog responses with the allowed licenses.
func catalogDraft(v string, obligation string, licenses ...string) []byte {
	enum, _ := json.Marshal(licenses)
	return []byte(`{"space":"` + testkit.Space + `","version":"` + v + `","slots":[{"slot":"odrl","obligation":"optional"},{"slot":"dcat","obligation":"` + obligation + `"}],
"files":[{"path":"schemas/catalog.schema.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["catalog_entry"],
"content":{"type":"object","properties":{"license":{"enum":` + string(enum) + `}},"required":["license"]}}]}`)
}

// catalogRoute is a shadow catalog route with passive response validation.
var catalogRoute = domain.Route{Listener: domain.DSPIngress, Match: domain.Match{Method: "POST", Path: "/catalog/request"}, Message: "CatalogRequestMessage",
	Gates: []domain.Gate{domain.GateCatalog}, Mode: domain.Enforce, ValidateResponse: &domain.ValidateResponse{Gate: domain.GateCatalog, Mode: domain.Shadow}}

// mgmtRoute is a management route validating the policy at $.policy.
var mgmtRoute = domain.Route{Listener: domain.MgmtIngress, Match: domain.Match{Method: "POST", Path: "/v3/policydefinitions"}, Message: "management",
	Gates: []domain.Gate{domain.GatePolicy}, Mode: domain.Enforce, Body: &domain.Body{Select: "$.policy"}}

func TestT_CLI_002_OnboardingRejectsInEnforce(t *testing.T) {
	_, c := f6Client(t)
	if r := request(c, "odrl:use"); r.Status != http.StatusBadRequest || testkit.BodyCode(r.Body) != "CLIENT_NOT_READY" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_002_OnboardingForwardsInShadow(t *testing.T) {
	s, _ := f6Client(t)
	c := testkit.NewClient(s, withMode(domain.Shadow))
	if r := request(c, "odrl:use"); r.Status != http.StatusOK || !c.Events.Has("CLIENT_NOT_READY") {
		t.Fatalf("got %d, events %v", r.Status, c.Events.Codes())
	}
}

func TestT_CLI_008_StateMachineCells(t *testing.T) {
	s, c := f6Client(t)
	state := func() domain.State { return domain.StateAt(c.Cell.Load(), s.Clock.Now()) }
	steps := []struct {
		name string
		do   func()
		want domain.State
	}{
		{"Onboarding+E2 stays", func() { c.Hub.Down = true; s.Clock.Set("2026-09-20T00:00:00Z"); _ = c.Sync.Run() }, domain.Onboarding},
		{"Onboarding+E1 one version", func() { c.Hub.Down = false; at(t, s, c, "2026-09-20T00:00:00Z") }, domain.Active},
		{"Active+E1 two versions", func() { at(t, s, c, "2026-10-04T00:00:00Z") }, domain.Transition},
		{"Transition+E2", func() { c.Hub.Down = true; s.Clock.Set("2026-10-05T00:00:00Z") }, domain.Degraded},
		{"Degraded+E2 stays", func() { s.Clock.Set("2026-10-06T00:00:00Z"); _ = c.Sync.Run() }, domain.Degraded},
		{"Degraded+E1", func() { c.Hub.Down = false; at(t, s, c, "2026-10-06T00:00:00Z") }, domain.Transition},
		{"Transition+E2 again", func() { c.Hub.Down = true; s.Clock.Set("2026-10-07T00:00:01Z") }, domain.Degraded},
		{"Degraded+E3", func() { s.Clock.Set("2026-10-14T00:00:00Z") }, domain.FailClosed},
		{"FailClosed+E3 stays", func() { s.Clock.Set("2026-10-20T00:00:00Z"); _ = c.Sync.Run() }, domain.FailClosed},
		{"FailClosed+E1", func() { c.Hub.Down = false; at(t, s, c, "2026-11-05T00:00:00Z") }, domain.Active},
		{"Active+E2", func() { c.Hub.Down = true; s.Clock.Set("2026-11-06T00:00:00Z") }, domain.Degraded},
	}
	for _, st := range steps {
		st.do()
		if got := state(); got != st.want {
			t.Fatalf("%s: got %s want %s", st.name, got, st.want)
		}
	}
}

func TestT_CLI_008_E2AtExactExpiry(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	exp := c.Cell.Load().ExpiresAt
	if domain.StateAt(c.Cell.Load(), exp-1) == domain.Degraded || domain.StateAt(c.Cell.Load(), exp) != domain.Degraded {
		t.Fatal("E2 must happen exactly at expires_at")
	}
	if domain.StateAt(c.Cell.Load(), exp.Add(7*timex.Day)) != domain.FailClosed || domain.StateAt(c.Cell.Load(), exp.Add(7*timex.Day-1)) != domain.Degraded {
		t.Fatal("E3 must happen exactly at expires_at + max_staleness")
	}
	_ = s
}

func TestT_CLI_018_DegradedContinueAndReject(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftWith(testkit.DraftV1, `{"degraded":{"policy_intake":"reject"}}`), app.ApproveOptions{})
	c := testkit.NewClient(s, testkit.F6Routes())
	at(t, s, c, "2026-09-10T00:00:00Z")
	c.Hub.Down = true
	s.Clock.Set("2026-09-11T00:00:00Z")
	if r := request(c, "odrl:use"); r.Status != http.StatusBadRequest || testkit.BodyCode(r.Body) != "CLIENT_DEGRADED_REJECT" {
		t.Fatalf("degraded reject: %d %s", r.Status, r.Body)
	}
	s2, c2 := synced(t, "2026-10-02T00:00:00Z")
	c2.Hub.Down = true
	s2.Clock.Set("2026-10-03T00:00:00Z")
	if r := request(c2, "odrl:use"); r.Status != http.StatusOK {
		t.Fatalf("degraded continue: %d %s", r.Status, r.Body)
	}
	s2.Clock.Set("2026-10-10T00:00:00Z")
	if r := request(c2, "odrl:use"); r.Status != http.StatusBadRequest || testkit.BodyCode(r.Body) != "CLIENT_FAIL_CLOSED" {
		t.Fatalf("fail closed: %d %s", r.Status, r.Body)
	}
}

func TestT_SYS_008_FailClosedRejectsEverything(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	c.Hub.Down = true
	s.Clock.Set("2026-10-10T00:00:00Z")
	for _, a := range []string{"odrl:use", "ex:access", "odrl:read"} {
		if r := request(c, a); r.Status != http.StatusBadRequest {
			t.Errorf("%s forwarded in FailClosed", a)
		}
	}
}

func TestT_CLI_010_ContextNotInPackageWithoutNetwork(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	doc := []byte(`{"space":"` + testkit.Space + `","version":"1.0.0","slots":[{"slot":"odrl","obligation":"mandatory"}],"files":[
{"path":"shapes.ttl","media_type":"text/turtle","role":"shacl","slot":"odrl","applies_to":["policy_intake"],"content":"@prefix sh: <http://www.w3.org/ns/shacl#> ."},
{"path":"ctx.jsonld","media_type":"application/ld+json","role":"context","slot":"odrl","applies_to":[],"url":"https://w3id.org/dspace/2025/1/context.jsonld","content":{"@context":{}}}]}`)
	s.Publish(t, "2026-09-01T10:00:00Z", doc, app.ApproveOptions{})
	whole := testkit.F6Routes()[:1]
	whole[0].Body = nil
	c := testkit.NewClient(s, whole)
	at(t, s, c, "2026-09-10T00:00:00Z")
	hub, did := c.Hub.Calls, c.Resolver.Calls
	evil := []byte(`{"@context":"https://evil.example/ctx.jsonld","@type":"ContractRequestMessage","offer":{"permission":[{"action":"odrl:use"}]}}`)
	r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", evil)
	if r.Status != http.StatusBadRequest || testkit.BodyCode(r.Body) != "CONTEXT_NOT_IN_PACKAGE" || c.Hub.Calls != hub || c.Resolver.Calls != did {
		t.Fatalf("got %d %s (hub calls %d→%d)", r.Status, r.Body, hub, c.Hub.Calls)
	}
	declared := []byte(`{"@context":"https://w3id.org/dspace/2025/1/context.jsonld","@type":"ContractRequestMessage"}`)
	if r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", declared); r.Status != http.StatusOK || !c.Events.Has("POC_EVAL_UNSUPPORTED_ROLE") {
		t.Fatalf("declared context rejected: %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_011_UnparseableAndNoVersion(t *testing.T) {
	s, c := f6Client(t)
	at(t, s, c, "2026-09-02T00:00:00Z")
	if r := request(c, "odrl:use"); testkit.BodyCode(r.Body) != "NO_VERSION_IN_FORCE" {
		t.Fatalf("scheduled only: %s", r.Body)
	}
	s.Clock.Set("2026-09-10T00:00:00Z")
	if r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", []byte(`{`)); testkit.BodyCode(r.Body) != "MESSAGE_UNPARSEABLE" {
		t.Fatalf("unparseable: %s", r.Body)
	}
	if r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", []byte(`{"offer":{}}`)); testkit.BodyCode(r.Body) != "MESSAGE_UNPARSEABLE" {
		t.Fatalf("selector without node: %s", r.Body)
	}
}

func TestT_CLI_011_RecommendedViolationIsInfo(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", catalogDraft("1.0.0", "recommended", "cc-by"), app.ApproveOptions{})
	c := testkit.NewClient(s, []domain.Route{catalogRoute})
	at(t, s, c, "2026-09-10T00:00:00Z")
	r := c.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{"license":"odc-by"}`))
	if r.Status != http.StatusOK || testkit.VerdictOf(r) != "" || !c.Events.Has("VERDICT_RECOMMENDED_NONCONFORMANT") {
		t.Fatalf("got %d %q %v", r.Status, testkit.VerdictOf(r), c.Events.Codes())
	}
}

func TestT_CLI_013_RejectBodiesPerMessageType(t *testing.T) {
	s, c := synced(t, "2026-11-05T12:00:00Z")
	r := request(c, "ex:access")
	var e map[string]any
	_ = json.Unmarshal(r.Body, &e)
	if r.Status != 400 || e["@type"] != "ContractNegotiationError" || e["consumerPid"] != "urn:uuid:c-1" || e["providerPid"] != nil || len(e["reason"].([]any)) != 1 {
		t.Fatalf("contract: %d %s", r.Status, r.Body)
	}
	c2 := testkit.NewClient(s, testkit.F6Routes())
	c2.Repo = c.Repo
	_ = c2.Configure(testkit.F6Routes(), "forward", "reject")
	at(t, s, c2, "2026-11-05T12:00:00Z")
	r = c2.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:none"))
	_ = json.Unmarshal(r.Body, &e)
	if r.Status != 400 || e["@type"] != "TransferError" || e["code"] != "AGREEMENT_UNKNOWN" || testkit.VerdictOf(r) == "" {
		t.Fatalf("transfer: %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_013_CatalogErrorAndManagementProblem(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	c := testkit.NewClient(s, []domain.Route{catalogRoute, mgmtRoute})
	c.Edge.Routes = []domain.Route{mgmtRoute}
	at(t, s, c, "2026-09-10T00:00:00Z")
	r := c.Send(domain.MgmtIngress, "POST", "/v3/policydefinitions", []byte(`{"policy":{"action":"odrl:read"}}`))
	var p map[string]any
	_ = json.Unmarshal(r.Body, &p)
	if r.Status != 422 || r.Header.Get("Content-Type") != "application/problem+json" || p["code"] != "VERDICT_VIOLATION" || len(p["findings"].([]any)) == 0 || testkit.VerdictOf(r) == "" {
		t.Fatalf("management: %d %s", r.Status, r.Body)
	}
	s2 := testkit.NewServer("2026-09-01T10:00:00Z")
	s2.Publish(t, "2026-09-01T10:00:00Z", catalogDraft("1.0.0", "mandatory", "cc-by"), app.ApproveOptions{})
	c2 := testkit.NewClient(s2, []domain.Route{catalogRoute})
	at(t, s2, c2, "2026-09-10T00:00:00Z")
	r = c2.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{"license":"x"}`))
	if r.Status != 400 || !strings.Contains(string(r.Body), `"CatalogError"`) {
		t.Fatalf("catalog: %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_013_ForwardAddsNoHeader(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	if r := request(c, "odrl:use"); r.Status != 200 || testkit.VerdictOf(r) != "" {
		t.Fatalf("got %d %q", r.Status, testkit.VerdictOf(r))
	}
}

func TestT_CLI_015_CaptureOnlyOn2xxBeforeReply(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	c.Upstream.Status = 500
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A0", "odrl:use", ""))
	if _, ok, _ := c.Repo.Get("urn:uuid:A0"); ok {
		t.Fatal("captured on a non-2xx response")
	}
	c.Upstream.Status = 201
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A2", "odrl:use", ""))
	if e, ok, _ := c.Repo.Get("urn:uuid:A2"); !ok || e.Version != "2.0.0" || e.Source != "dsp-egress" || e.ProviderPID != "urn:uuid:p-1" {
		t.Fatalf("2xx capture: %+v", e)
	}
}

func TestT_CLI_015_ShadowWithoutPassIsUnverifiedHead(t *testing.T) {
	s, _ := synced(t, "2026-11-05T12:00:00Z")
	c := testkit.NewClient(s, withMode(domain.Shadow))
	at(t, s, c, "2026-11-05T12:00:00Z")
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A3", "ex:other", ""))
	if e, ok, _ := c.Repo.Get("urn:uuid:A3"); !ok || e.Source != "unverified" || e.Version != "2.0.0" {
		t.Fatalf("got %+v", e)
	}
}

func TestT_CLI_015_ProfileMismatch(t *testing.T) {
	_, c := synced(t, "2026-11-05T12:00:00Z")
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A4", "odrl:use", "https://example.com/profiles/odrl/9.9.9"))
	if !c.Events.Has("AGREEMENT_PROFILE_MISMATCH") {
		t.Fatal("mismatch not reported")
	}
	c.Events.Reset()
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A5", "odrl:use", "https://example.com/profiles/odrl/2.0.0"))
	if c.Events.Has("AGREEMENT_PROFILE_MISMATCH") {
		t.Fatal("fixed profile reported as mismatch")
	}
}

func TestT_CLI_016_AdmissionRows(t *testing.T) {
	s, c := synced(t, "2026-11-05T12:00:00Z")
	_ = c.Repo.Put(domain.Entry{AgreementID: "urn:uuid:A1", Version: "1.0.0", Manifest: manifestOf(t, s, "2.0.0")})
	transfer := func(id string) *clientapp.Response {
		return c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest(id))
	}
	if r := transfer("urn:uuid:A1"); r.Status != 200 {
		t.Fatalf("registered: %d %s", r.Status, r.Body)
	}
	if r := transfer("urn:uuid:nope"); r.Status != 200 || !c.Events.Has("AGREEMENT_UNKNOWN") {
		t.Fatalf("unknown forward: %d", r.Status)
	}
	_ = c.Configure(testkit.F6Routes(), "forward", "reject")
	at(t, s, c, "2026-11-05T12:00:00Z")
	if r := transfer("urn:uuid:nope"); r.Status != 400 || testkit.BodyCode(r.Body) != "AGREEMENT_UNKNOWN" {
		t.Fatalf("unknown reject: %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_016_RevocationScheduledPerAgreement(t *testing.T) {
	s, c := synced(t, "2026-10-10T12:00:00Z")
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A1", "ex:access", ""))
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-2/agreement", testkit.AgreementMessage("urn:uuid:A2", "ex:access", ""))
	at(t, s, c, "2026-11-05T12:00:00Z")
	s.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
	at(t, s, c, "2026-11-10T00:00:00Z")
	if n := c.Events.Count("AGREEMENT_REVOCATION_SCHEDULED"); n != 2 {
		t.Fatalf("got %d events", n)
	}
}

func TestT_CLI_017_RetiredVersionWithAgreementIsRetained(t *testing.T) {
	s, c := synced(t, "2026-10-10T12:00:00Z")
	c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A1", "ex:access", ""))
	at(t, s, c, "2026-11-05T12:00:00Z")
	v1 := manifestOf(t, s, "2.0.0")
	_ = v1
	var role string
	for _, v := range c.Status.Report().Versions {
		if v.Version == "1.0.0" {
			role = v.LocalRole
		}
	}
	e, _, _ := c.Repo.Get("urn:uuid:A1")
	if _, held, _ := c.Packages.Get(e.Manifest); role != "retained" || !held {
		t.Fatalf("role %q, package held %v", role, held)
	}
}

func TestT_CLI_012_ShadowForwardsWithEventObserveIsSilent(t *testing.T) {
	s, _ := synced(t, "2026-11-05T12:00:00Z")
	for _, m := range []domain.Mode{domain.Shadow, domain.Observe} {
		c := testkit.NewClient(s, withMode(m))
		at(t, s, c, "2026-11-05T12:00:00Z")
		r := request(c, "ex:access")
		events := len(c.Events.Verdicts())
		if r.Status != 200 || testkit.VerdictOf(r) != "" || (m == domain.Shadow) != (events > 0) {
			t.Errorf("%s: %d %q events=%d", m, r.Status, testkit.VerdictOf(r), events)
		}
	}
}

func TestT_CLI_020_StatusFields(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	b, _ := json.Marshal(c.Status.Report())
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	for _, k := range []string{"space", "state", "versions", "last_accepted_listing", "hub_did", "clock_skew_estimate_ms", "gates"} {
		if _, ok := m[k]; !ok {
			t.Errorf("missing %s in %s", k, b)
		}
	}
	if m["state"] != "Transition" || !strings.Contains(string(b), `"local_role":"head"`) || !strings.Contains(string(b), `"local_role":"in_force"`) {
		t.Fatalf("got %s", b)
	}
}

func TestT_CLI_021_VerdictEventsValidate(t *testing.T) {
	s, c := f6Client(t)
	request(c, "odrl:use")
	at(t, s, c, "2026-10-10T12:00:00Z")
	for _, a := range []string{"odrl:use", "ex:access", "odrl:read", "nope"} {
		request(c, a)
	}
	c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:x"))
	vs := c.Events.Verdicts()
	if len(vs) < 4 {
		t.Fatalf("only %d verdict events", len(vs))
	}
	for _, v := range vs {
		b, _ := json.Marshal(v)
		if err := schemas.Validate(schemas.Verdict, b); err != nil {
			t.Errorf("%s: %v", b, err)
		}
	}
}

func TestT_CLI_022_DefaultAction(t *testing.T) {
	s, c := synced(t, "2026-10-10T12:00:00Z")
	if r := c.Send(domain.DSPIngress, "POST", "/unknown", []byte(`{}`)); r.Status != 200 || len(c.Upstream.Received) != 1 {
		t.Fatalf("forward: %d", r.Status)
	}
	_ = c.Configure(testkit.F6Routes(), "reject", "forward")
	at(t, s, c, "2026-10-10T12:00:00Z")
	if r := c.Send(domain.DSPIngress, "POST", "/unknown", []byte(`{}`)); r.Status != 400 || testkit.BodyCode(r.Body) != "ROUTE_DEFAULT_REJECT" {
		t.Fatalf("reject: %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_023_PassiveAudit(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", catalogDraft("1.0.0", "mandatory", "cc-by", "odc-by"), app.ApproveOptions{})
	c := testkit.NewClient(s, []domain.Route{catalogRoute})
	at(t, s, c, "2026-09-10T00:00:00Z")
	c.Upstream.Body = []byte(`{"license":"odc-by"}`)
	c.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{"license":"cc-by"}`))
	s.Publish(t, "2026-10-01T10:00:00Z", catalogDraft("2.0.0", "mandatory", "cc-by"), app.ApproveOptions{})
	at(t, s, c, "2026-10-01T10:00:00Z")
	rep := c.Audit.Latest()
	if !c.Events.Has("AUDIT_WILL_WARN") || !c.Events.Has("AUDIT_WILL_REJECT") || rep == nil || len(rep.Entries) != 2 {
		t.Fatalf("events %v report %+v", c.Events.Codes(), rep)
	}
}

func TestT_CLI_026_ImportAndGetNotFound(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	err := c.Agreements.Import([]clientapp.ImportPair{{AgreementID: "urn:x", Version: "7.0.0"}}, c.Cell.Load())
	if ce, ok := codes.As(err); !ok || ce.Code != codes.ApiNotFound {
		t.Fatalf("import: %v", err)
	}
	_, err = c.Agreements.Get("urn:none", c.Cell.Load(), c.Clock.Now())
	if ce, ok := codes.As(err); !ok || ce.Code != codes.ApiNotFound {
		t.Fatalf("get: %v", err)
	}
}

// corpus is a fixed list of (instant, action) pairs covering every F6 frontier.
var corpus = []struct{ at, action string }{
	{"2026-10-02T12:00:00Z", "odrl:read"}, {"2026-10-03T09:59:49Z", "ex:access"}, {"2026-10-03T09:59:50Z", "ex:access"},
	{"2026-10-10T12:00:00Z", "ex:access"}, {"2026-11-02T10:00:09Z", "ex:access"}, {"2026-11-02T10:00:10Z", "ex:access"}, {"2026-11-02T10:00:10Z", "odrl:read"},
}

// replay runs the corpus on a client and returns comparable outcomes.
func replay(t *testing.T) []string {
	t.Helper()
	s, c := f6Client(t)
	at(t, s, c, "2026-10-02T00:00:00Z")
	var out []string
	for _, p := range corpus {
		s.Clock.Set(p.at)
		if !(s.Clock.Now() < c.Cell.Load().ExpiresAt) || p.at == "2026-11-02T10:00:09Z" {
			at(t, s, c, "2026-11-02T06:00:00Z")
			s.Clock.Set(p.at)
		}
		r := request(c, p.action)
		out = append(out, p.at+" "+p.action+" "+http.StatusText(r.Status)+" "+testkit.VerdictOf(r)+" "+testkit.BodyCode(r.Body))
	}
	return out
}

func TestT_SYS_001_TwoClientsSameOutputs(t *testing.T) {
	a, b := replay(t), replay(t)
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("outputs differ:\n%v\n%v", a, b)
	}
}

func TestT_SYS_002_EdgeHasNoHubAccess(t *testing.T) {
	hubAPI := reflect.TypeOf((*clientapp.HubAPI)(nil)).Elem()
	resolver := reflect.TypeOf((*clientapp.DIDResolver)(nil)).Elem()
	et := reflect.TypeOf(clientapp.Edge{})
	for i := 0; i < et.NumField(); i++ {
		f := et.Field(i).Type
		if f.Implements(hubAPI) || f.Implements(resolver) || (f.Kind() == reflect.Interface && (hubAPI.Implements(f) || resolver.Implements(f))) {
			t.Errorf("Edge.%s can reach the hub (%s)", et.Field(i).Name, f)
		}
	}
	s, c := synced(t, "2026-10-10T12:00:00Z")
	hub, did := c.Hub.Calls, c.Resolver.Calls
	for _, a := range []string{"odrl:use", "ex:access", "odrl:read", "x"} {
		request(c, a)
	}
	if c.Hub.Calls != hub || c.Resolver.Calls != did {
		t.Fatal("the hot path called the hub")
	}
	_ = s
}

func TestT_SYS_004_FrontiersComeFromListing(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	_, l := s.LatestListing(t)
	nc := l.Versions[1].NewContentFrom
	for _, d := range []timex.Duration{-11, -10} {
		s.Clock.Set(nc.Add(d).String())
		got := len(c.Cell.Load().InForce(s.Clock.Now()))
		if want := map[timex.Duration]int{-11: 1, -10: 2}[d]; got != want {
			t.Errorf("at new_content_from%+ds: %d versions", d, got)
		}
	}
}

func TestT_SYS_007_VersionRejectsOnlyAfterEnforcedPlusLease(t *testing.T) {
	s, c := synced(t, "2026-11-02T06:00:00Z")
	s.Clock.Set("2026-11-02T10:00:09Z")
	if r := request(c, "ex:access"); r.Status != 200 {
		t.Fatalf("rejected before ef+L: %s", r.Body)
	}
	s.Clock.Set("2026-11-02T10:00:10Z")
	if r := request(c, "ex:access"); r.Status != 400 {
		t.Fatal("not rejected at ef+L")
	}
}
