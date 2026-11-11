// Tests of the admin API over HTTP: /status, /readyz by state, rules CRUD with If-Match, :explain without effects and API_NOT_FOUND answers.
// Fixtures: an F6 client from testkit served by httptest (T-CLI-020, 026, 028, 032).
package status_http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/adapters/status_http"
	"github.com/ging/alexandria/internal/testkit"
)

// admin serves the admin API of an F6 client, synced at ts when given.
func admin(t *testing.T, ts string) (*httptest.Server, *testkit.Client) {
	t.Helper()
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	if ts != "" {
		s.Clock.Set(ts)
		c.SyncNow(t)
	}
	srv := httptest.NewServer(status_http.Admin{Status: c.Status, Rules: c.Rules, Explainer: &c.Edge.Explainer, Agreements: c.Agreements, Audit: c.Audit, Cell: c.Cell, Metrics: metrics.New()}.Handler())
	t.Cleanup(srv.Close)
	return srv, c
}

// do sends a request with optional If-Match and returns status, ETag and body.
func do(t *testing.T, srv *httptest.Server, method, path, ifMatch, body string) (int, string, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, srv.URL+path, strings.NewReader(body))
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header.Get("ETag"), string(b)
}

// rule is a valid new rule on dsp-ingress.
const rule = `{"id":"extra","priority":999,"listener":"dsp-ingress","match":{"method":"POST","path":"/extra"},"message":"ContractRequestMessage","verify":{"gates":["dsp_message"]}}`

func TestT_CLI_020_StatusAndReadiness(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	if st, _, b := do(t, srv, "GET", "/status", "", ""); st != 200 || !strings.Contains(b, `"state":"Transition"`) || !strings.Contains(b, `"clock_skew_estimate_ms"`) {
		t.Fatalf("%d %s", st, b)
	}
	if st, _, _ := do(t, srv, "GET", "/readyz", "", ""); st != 200 {
		t.Fatalf("readyz %d", st)
	}
}

func TestReadyzIs503WhileOnboarding(t *testing.T) {
	srv, _ := admin(t, "")
	if st, _, _ := do(t, srv, "GET", "/readyz", "", ""); st != 503 {
		t.Fatalf("readyz %d", st)
	}
}

func TestT_CLI_026_NotFound(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	for _, p := range []string{"/agreements/urn:none", "/rules/none"} {
		if st, _, b := do(t, srv, "GET", p, "", ""); st != 404 || !strings.Contains(b, "API_NOT_FOUND") {
			t.Errorf("%s: %d %s", p, st, b)
		}
	}
}

func TestT_CLI_028_IfMatchRequiredAndChecked(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	if st, _, b := do(t, srv, "POST", "/rules", "", rule); st != 428 || !strings.Contains(b, "API_PRECONDITION_REQUIRED") {
		t.Fatalf("%d %s", st, b)
	}
	if st, _, b := do(t, srv, "POST", "/rules", `"0"`, rule); st != 412 || !strings.Contains(b, "API_PRECONDITION_FAILED") {
		t.Fatalf("%d %s", st, b)
	}
}

func TestT_CLI_028_AddThenConcurrentChangeFails(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	_, etag, _ := do(t, srv, "GET", "/rules", "", "")
	if st, next, b := do(t, srv, "POST", "/rules", etag, rule); st != 201 || next != `"2"` {
		t.Fatalf("%d %s %s", st, next, b)
	}
	if st, _, _ := do(t, srv, "DELETE", "/rules/extra", etag, ""); st != 412 {
		t.Fatalf("stale delete: %d", st)
	}
}

func TestT_CLI_028_DuplicateIDLeavesSetUnchanged(t *testing.T) {
	srv, c := admin(t, "2026-10-10T12:00:00Z")
	dup := strings.Replace(rule, `"extra"`, `"contract-request"`, 1)
	if st, _, b := do(t, srv, "POST", "/rules", `"1"`, dup); st != 422 || !strings.Contains(b, "RULES_DUPLICATE_ID") {
		t.Fatalf("%d %s", st, b)
	}
	if c.Rules.Get().Revision != 1 {
		t.Fatal("a rejected change altered the set")
	}
}

func TestT_CLI_032_ExplainOverHTTP(t *testing.T) {
	srv, c := admin(t, "2026-10-10T12:00:00Z")
	body := `{"listener":"dsp-ingress","method":"POST","path":"` + testkit.ContractRequestPath + `","body":` + string(testkit.ContractRequest("ex:access")) + `}`
	st, _, b := do(t, srv, "POST", "/rules:explain", "", body)
	if st != 200 || !strings.Contains(b, `"action":"forward-notice"`) || !strings.Contains(b, "VERDICT_NOT_IN_HEAD") || c.Upstream.Count() != 0 {
		t.Fatalf("%d %s", st, b)
	}
}
