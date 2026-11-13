// Tests of the validator listener over HTTP: an ext_authz-style check gets 200 or the reject to relay, and nothing reaches the connector.
// Fixtures: a synced F6 client from testkit behind httptest, standing where Envoy would call (§4.8).
package authz_http_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/authz_http"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/testkit"
)

// setup returns a synced F6 client at an instant and its dsp-ingress validator listener.
func setup(t *testing.T, at string) (*testkit.Client, *httptest.Server) {
	t.Helper()
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	s.Clock.Set(at)
	c.SyncNow(t)
	srv := httptest.NewServer(authz_http.Handler{Edge: c.Edge, Listener: domain.DSPIngress})
	t.Cleanup(srv.Close)
	return c, srv
}

// ask sends the check a proxy would send for an F6 contract request with action.
func ask(t *testing.T, base, action string) (int, http.Header, []byte) {
	t.Helper()
	req, _ := http.NewRequestWithContext(t.Context(), http.MethodPost, base+testkit.ContractRequestPath, bytes.NewReader(testkit.ContractRequest(action)))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

func TestAllowedCheckIsAnEmpty200(t *testing.T) {
	c, srv := setup(t, "2026-10-10T12:00:00Z")
	st, h, b := ask(t, srv.URL, "odrl:use")
	if st != http.StatusOK || len(b) != 0 || h.Get(domain.HeaderName) != "" || c.Upstream.Count() != 0 {
		t.Fatalf("got %d %q %v with %d forwards", st, b, h, c.Upstream.Count())
	}
}

func TestWarningCheckCarriesVerdictHeader(t *testing.T) {
	_, srv := setup(t, "2026-10-10T12:00:00Z")
	st, h, _ := ask(t, srv.URL, "ex:access")
	if st != http.StatusOK || !testkit.Contains(h.Get(domain.HeaderName), "VERDICT_NOT_IN_HEAD") {
		t.Fatalf("got %d %q", st, h.Get(domain.HeaderName))
	}
}

func TestRejectedCheckCarriesDSPError(t *testing.T) {
	c, srv := setup(t, "2026-11-05T12:00:00Z")
	st, h, b := ask(t, srv.URL, "ex:access")
	if st != http.StatusBadRequest || testkit.BodyCode(b) != "VERDICT_VIOLATION" || h.Get("Content-Type") != "application/json" || h.Get(domain.HeaderName) == "" || c.Upstream.Count() != 0 {
		t.Fatalf("got %d %s %v", st, b, h)
	}
}
