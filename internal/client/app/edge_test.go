// Tests the hot path against the in-process hub: F6.3 rows, F6.4 agreements and revocation, responses to the sender, events and :explain.
// Fixtures: the F6 hub and client of sync_test.go, and the testkit F6 messages (T-SYS-001..004, 007, 008, T-CLI-002, 010–017, 021, 029, 032, 033).
package app_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
	srvapp "github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// at moves the shared clock and syncs.
func at(t *testing.T, s *testkit.Server, c *testkit.Client, ts string) {
	t.Helper()
	s.Clock.Set(ts)
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	c.SyncNow(t)
}

// request sends an F6 contract request with action.
func request(c *testkit.Client, action string) *app.Response {
	return c.Send(domain.DSPIngress, "POST", testkit.ContractRequestPath, testkit.ContractRequest(action))
}

// m2Header is the exact F6 header of m2 during the coexistence.
const m2Header = `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";rule="contract-request";version="1.0.0";sunset=@1793613600`

func TestT_SYS_007_F6Rows(t *testing.T) {
	s, c := f6(t)
	// The client syncs before each barrier, as in F6: the hub drops 1.0.0 from versions[] at enforced_from (OPEN_QUESTIONS).
	rows := []struct {
		sync, ts   string
		m1, m2, m3 int
	}{
		{"2026-10-02T12:00:00Z", "2026-10-02T12:00:00Z", 200, 200, 400},
		{"2026-10-03T06:00:00Z", "2026-10-03T09:59:55Z", 200, 200, 200},
		{"2026-10-10T12:00:00Z", "2026-10-10T12:00:00Z", 200, 200, 200},
		{"2026-11-02T06:00:00Z", "2026-11-02T10:00:05Z", 200, 200, 200},
		{"2026-11-05T12:00:00Z", "2026-11-05T12:00:00Z", 200, 400, 200},
	}
	for _, r := range rows {
		at(t, s, c, r.sync)
		s.Clock.Set(r.ts)
		for action, want := range map[string]int{"odrl:use": r.m1, "ex:access": r.m2, "odrl:read": r.m3} {
			if got := request(c, action).Status; got != want {
				t.Errorf("%s %s: %d want %d", r.ts, action, got, want)
			}
		}
	}
}

func TestT_CLI_013_WarningHeaderExact(t *testing.T) {
	_, c := f6(t)
	if h := testkit.VerdictOf(request(c, "ex:access")); h != m2Header {
		t.Fatalf("got %q", h)
	}
}

func TestT_CLI_013_RejectBodyIsContractNegotiationError(t *testing.T) {
	s, c := f6(t)
	at(t, s, c, "2026-11-05T12:00:00Z")
	r := request(c, "ex:access")
	var body map[string]any
	_ = json.Unmarshal(r.Body, &body)
	if r.Status != 400 || body["@type"] != "ContractNegotiationError" || body["code"] != "VERDICT_VIOLATION" || body["providerPid"] != "urn:uuid:p-1" || testkit.VerdictOf(r) == "" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_014_ForwardedBytesAreIdentical(t *testing.T) {
	_, c := f6(t)
	body := testkit.ContractRequest("odrl:use")
	c.Send(domain.DSPIngress, "POST", testkit.ContractRequestPath, body)
	if got := c.Upstream.Received[0].Body; !bytes.Equal(got, body) {
		t.Fatal("forwarded body differs")
	}
}

func TestT_CLI_002_BeforeFirstSyncNotReady(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	if r := request(c, "odrl:use"); r.Status != 400 || testkit.BodyCode(r.Body) != "CLIENT_NOT_READY" {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestT_SYS_008_FailClosedRejects(t *testing.T) {
	s, c := f6(t)
	s.Clock.Set("2026-10-18T12:00:01Z")
	if r := request(c, "odrl:use"); testkit.BodyCode(r.Body) != "CLIENT_FAIL_CLOSED" {
		t.Fatalf("got %s", r.Body)
	}
}

func TestT_SYS_002_HotPathNeverCallsTheHub(t *testing.T) {
	_, c := f6(t)
	calls := c.Hub.Calls
	for i := 0; i < 10; i++ {
		request(c, "ex:access")
	}
	if c.Hub.Calls != calls {
		t.Fatal("the hot path called the hub")
	}
}

func TestT_SYS_001_TwoClientsDecideTheSame(t *testing.T) {
	s, a := f6(t)
	b := testkit.NewClient(t, s)
	b.SyncNow(t)
	for _, ts := range []string{"2026-10-10T12:00:00Z", "2026-10-12T12:00:00Z", "2026-11-05T12:00:00Z"} {
		at(t, s, a, ts)
		b.SyncNow(t)
		for _, act := range []string{"odrl:use", "ex:access", "odrl:read"} {
			x, y := request(a, act), request(b, act)
			if x.Status != y.Status || testkit.VerdictOf(x) != testkit.VerdictOf(y) || testkit.BodyCode(x.Body) != testkit.BodyCode(y.Body) {
				t.Fatalf("%s %s differ", ts, act)
			}
		}
	}
}

func TestT_CLI_021_VerdictEventsValidate(t *testing.T) {
	_, c := f6(t)
	request(c, "ex:access")
	vs := c.Events.Verdicts()
	if len(vs) == 0 {
		t.Fatal("no vhub.verdict")
	}
	for _, v := range vs {
		b, _ := json.Marshal(v)
		if err := schemas.Validate(schemas.Verdict, b); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
	}
}

func TestT_CLI_010_RemoteContextNotInPackage(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	doc := []byte(`{"version":"1.0.0","slots":[{"slot":"odrl","obligation":"mandatory","applies_to":["policy_intake"]}],"files":[{"path":"odrl/shapes.ttl","media_type":"text/turtle","role":"shacl","slot":"odrl","applies_to":["policy_intake"],"content":"@prefix sh: <http://www.w3.org/ns/shacl#> ."}]}`)
	s.Publish(t, "2026-09-01T10:00:00Z", doc, svOpts)
	s.Clock.Set("2026-09-10T00:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	body := []byte(`{"@context":"https://evil.example/ctx.jsonld","@type":"ContractRequestMessage","providerPid":"urn:uuid:p-1"}`)
	r := c.Send(domain.DSPIngress, "POST", testkit.ContractRequestPath, body)
	if testkit.BodyCode(r.Body) != "CONTEXT_NOT_IN_PACKAGE" || c.Hub.Calls != 1 {
		t.Fatalf("got %s with %d hub calls", r.Body, c.Hub.Calls)
	}
}

func TestT_CLI_015_AgreementRegisteredWithPassingVersion(t *testing.T) {
	_, c := f6(t)
	c.SetMode(t, "contract-agreement-in", domain.Enforce)
	c.Send(domain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	e, ok, _ := c.Repo.Get("urn:A1")
	if !ok || e.Version != "1.0.0" || e.Role != "consumer" || e.Source != "dsp-ingress" {
		t.Fatalf("got %+v", e)
	}
}

func TestT_CLI_015_NoCaptureOnUpstreamError(t *testing.T) {
	_, c := f6(t)
	c.Upstream.Status = 500
	c.Send(domain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	if _, ok, _ := c.Repo.Get("urn:A1"); ok {
		t.Fatal("agreement captured on a 500")
	}
}

func TestT_CLI_015_ObserveCapturesUnverifiedHead(t *testing.T) {
	_, c := f6(t)
	c.Send(domain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	if e, _, _ := c.Repo.Get("urn:A1"); e.Source != "unverified" || e.Version != "2.0.0" {
		t.Fatalf("got %+v", e)
	}
}

func TestT_CLI_015_ProfileMismatch(t *testing.T) {
	_, c := f6(t)
	c.Send(domain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", "https://other/profile"))
	if !c.Events.Has(string(codes.AgreementProfileMismatch)) {
		t.Fatal("AGREEMENT_PROFILE_MISMATCH not emitted")
	}
}

// a1 registers A1 on 2026-10-10 and enforces transfers, as in F6.4.
func a1(t *testing.T) (*testkit.Server, *testkit.Client) {
	t.Helper()
	s, c := f6(t)
	c.SetMode(t, "contract-agreement-in", domain.Enforce)
	c.SetMode(t, "transfer-request", domain.Enforce)
	c.Send(domain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	return s, c
}

// transfer sends a transfer request for A1.
func transfer(c *testkit.Client) *app.Response {
	return c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:A1"))
}

func TestT_CLI_016_RetiredVersionStillAdmits(t *testing.T) {
	s, c := a1(t)
	at(t, s, c, "2026-11-05T12:00:00Z")
	if r := transfer(c); r.Status != 200 {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_016_RevocationScheduledAndEnforced(t *testing.T) {
	s, c := a1(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", revoke); err != nil {
		t.Fatal(err)
	}
	c.SyncNow(t)
	if !c.Events.Has(string(codes.AgreementRevocationScheduled)) {
		t.Fatal("AGREEMENT_REVOCATION_SCHEDULED not emitted")
	}
	s.Clock.Set("2026-11-12T00:00:05Z")
	if r := transfer(c); r.Status != 200 {
		t.Fatalf("before revoked_from+L: %d", r.Status)
	}
	s.Clock.Set("2026-11-12T00:00:10Z")
	r := transfer(c)
	var body map[string]any
	_ = json.Unmarshal(r.Body, &body)
	if r.Status != 400 || body["@type"] != "TransferError" || body["code"] != "AGREEMENT_VERSION_REVOKED" || body["providerPid"] != nil {
		t.Fatalf("got %d %s", r.Status, r.Body)
	}
}

func TestT_CLI_016_UnknownAgreementReject(t *testing.T) {
	s, _ := f6(t)
	c := testkit.NewClientWith(t, s, testkit.SeedPath(t), "forward", "reject")
	c.SyncNow(t)
	c.SetMode(t, "transfer-request", domain.Enforce)
	if r := transfer(c); testkit.BodyCode(r.Body) != "AGREEMENT_UNKNOWN" {
		t.Fatalf("got %s", r.Body)
	}
}

func TestT_CLI_017_AgreedPackageRetainedAfterRetirement(t *testing.T) {
	s, c := a1(t)
	at(t, s, c, "2026-11-05T12:00:00Z")
	c.GC.Run()
	roles := map[string]string{}
	for _, v := range c.Status.Get().Versions {
		roles[v.Version] = v.LocalRole
	}
	if roles["1.0.0"] != "retained" {
		t.Fatalf("roles %v", roles)
	}
}

func TestT_CLI_033_GCDeletesUnlistedWithoutAgreement(t *testing.T) {
	s, c := f6(t)
	at(t, s, c, "2026-11-05T12:00:00Z")
	deleted := c.GC.Run()
	if len(deleted) != 1 || deleted[0] != "1.0.0" || len(c.Events.Notices(app.EvGC)) != 1 {
		t.Fatalf("deleted %v", deleted)
	}
}

func TestT_CLI_032_ExplainMatchesHotPathWithoutEffects(t *testing.T) {
	_, c := f6(t)
	x := c.Edge.Explain(&app.Request{Listener: domain.DSPIngress, Method: "POST", Path: testkit.ContractRequestPath, Body: testkit.ContractRequest("ex:access")})
	if x.Action != "forward-notice" || x.Verdict != m2Header || *x.Rule != "contract-request" || len(x.Gates) != 2 {
		t.Fatalf("got %+v", x)
	}
	if c.Upstream.Count() != 0 || len(c.Events.Verdicts()) != 0 {
		t.Fatal("explain had effects")
	}
}

func TestT_CLI_029_MessageUsesOneRevision(t *testing.T) {
	_, c := f6(t)
	request(c, "ex:access")
	c.SetMode(t, "negotiation-event", domain.Shadow)
	request(c, "ex:access")
	vs := c.Events.Verdicts()
	if vs[0].RulesRevision != 1 || vs[len(vs)-1].RulesRevision != 2 || len(c.Events.Notices(app.EvRules)) != 1 {
		t.Fatalf("revisions %d..%d", vs[0].RulesRevision, vs[len(vs)-1].RulesRevision)
	}
}

func TestT_CLI_023_PassiveAuditWarnsAhead(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	catalog := func(v string, actions ...string) []byte {
		return []byte(`{"version":"` + v + `","slots":[{"slot":"dcat","obligation":"mandatory","applies_to":["catalog_entry"]}],"files":[{"path":"c.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["catalog_entry"],"content":{"properties":{"action":{"enum":` + list(actions) + `}}}}]}`)
	}
	s.Publish(t, "2026-09-01T10:00:00Z", catalog("1.0.0", "a", "b"), svOpts)
	s.Clock.Set("2026-09-10T00:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	c.Upstream.Body = []byte(`{"action":"b"}`)
	c.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{"@type":"CatalogRequestMessage"}`))
	s.Publish(t, "2026-09-20T10:00:00Z", catalog("2.0.0", "a"), svOpts)
	c.SyncNow(t)
	if !c.Events.Has(string(codes.AuditWillReject)) || c.Audit.Latest() == nil {
		t.Fatal("AUDIT_WILL_REJECT not emitted")
	}
}

func TestT_CLI_025_ClockSkewExceeded(t *testing.T) {
	_, c := f6(t)
	c.Status.ObserveSkew(c.Server.Clock.Now()-30, c.Server.Clock.Now())
	if !c.Events.Has(string(codes.ClockSkewExceeded)) || c.Status.Get().ClockSkewEstimateMS != 30000 {
		t.Fatal("CLOCK_SKEW_EXCEEDED not emitted")
	}
}

func TestT_CLI_020_StatusFields(t *testing.T) {
	_, c := f6(t)
	st := c.Status.Get()
	if st.Namespace != testkit.NSIRI || st.State != domain.Transition || st.LastAcceptedListing == nil || st.HubDID.ResolvedAt == nil || st.Rules.Count != 16 || len(st.Versions) != 2 {
		t.Fatalf("got %+v", st)
	}
}

func TestT_CLI_031_RulesSurviveRestartAndSeedOnlyWhenEmpty(t *testing.T) {
	_, c := f6(t)
	c.SetMode(t, "negotiation-event", domain.Shadow)
	c.Restart(t, "file:///no/existe")
	if rs := c.Rules.Get(); rs.Revision != 2 {
		t.Fatalf("revision %d after restart", rs.Revision)
	}
}

// svOpts approves with the default grace.
var svOpts = srvapp.ApproveOptions{}

// list renders a JSON array of strings.
func list(xs []string) string { b, _ := json.Marshal(xs); return string(b) }
