// Tests of the admin API over HTTP: /status fields, /readyz by state, /metrics format and API_NOT_FOUND answers.
// Realise T-CLI-020 and T-CLI-026 over HTTP (§4.11).
package status_http_test

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/adapters/status_http"
	"github.com/ging/alexandria/internal/testkit"
)

// admin serves the admin API of a client synced at ts.
func admin(t *testing.T, ts string) (*httptest.Server, *testkit.Client) {
	t.Helper()
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(s, testkit.F6Routes())
	if ts != "" {
		s.Clock.Set(ts)
		_ = c.Sync.Run()
	}
	srv := httptest.NewServer(status_http.Admin{Status: c.Status, Agreements: c.Agreements, Audit: c.Audit, Cell: c.Cell, Metrics: metrics.New()}.Handler())
	t.Cleanup(srv.Close)
	return srv, c
}

// get fetches a path and returns status and body.
func get(t *testing.T, srv *httptest.Server, method, path, body string) (int, string) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), method, srv.URL+path, strings.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestT_CLI_020_StatusOverHTTP(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	st, b := get(t, srv, "GET", "/status", "")
	var m map[string]any
	if st != 200 || json.Unmarshal([]byte(b), &m) != nil || m["state"] != "Transition" || m["last_accepted_listing"] == nil {
		t.Fatalf("got %d %s", st, b)
	}
}

func TestReadyzFollowsState(t *testing.T) {
	srv, _ := admin(t, "")
	if st, _ := get(t, srv, "GET", "/readyz", ""); st != 503 {
		t.Fatalf("onboarding: %d", st)
	}
	srv, _ = admin(t, "2026-10-10T12:00:00Z")
	if st, _ := get(t, srv, "GET", "/readyz", ""); st != 200 {
		t.Fatalf("transition: %d", st)
	}
}

func TestT_CLI_026_NotFoundOverHTTP(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	if st, b := get(t, srv, "GET", "/agreements/urn:uuid:none", ""); st != 404 || !strings.Contains(b, `"API_NOT_FOUND"`) {
		t.Fatalf("get: %d %s", st, b)
	}
	if st, b := get(t, srv, "POST", "/agreements:import", `[{"agreement_id":"urn:x","version":"9.0.0"}]`); st != 404 || !strings.Contains(b, `"API_NOT_FOUND"`) {
		t.Fatalf("import: %d %s", st, b)
	}
	if st, _ := get(t, srv, "POST", "/agreements:import", `[{"agreement_id":"urn:x","version":"1.0.0"}]`); st != 200 {
		t.Fatalf("import known: %d", st)
	}
}

func TestMetricsAreOpenMetrics(t *testing.T) {
	srv, _ := admin(t, "2026-10-10T12:00:00Z")
	st, b := get(t, srv, "GET", "/metrics", "")
	if st != 200 || !strings.HasSuffix(b, "# EOF\n") || !strings.Contains(b, "vhub_listing_age_seconds") {
		t.Fatalf("got %d %s", st, b)
	}
}
