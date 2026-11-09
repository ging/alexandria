// Tests of the L7 proxy: requests and responses cross it byte for byte, egress captures agreements, admin stays off DSP.
// Realise T-SYS-003, T-CLI-014 and T-CLI-019 with a recording connector behind httptest (§4.2, §4.8).
package proxy_dsp_test

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/proxy_dsp"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/testkit"
)

// capture records the last request seen by a handler.
type capture struct {
	mu     sync.Mutex
	header http.Header
	body   []byte
	host   string
	path   string
}

// wrap records the request before calling next.
func (c *capture) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		c.mu.Lock()
		c.header, c.body, c.host, c.path = r.Header.Clone(), b, r.Host, r.URL.Path
		c.mu.Unlock()
		r.Body = io.NopCloser(bytes.NewReader(b))
		next.ServeHTTP(w, r)
	})
}

// connectorBody is the fixed answer of the fake connector, with odd spacing to detect re-serialization.
var connectorBody = []byte("{ \"@type\" : \"ContractNegotiation\",\n  \"state\":\"REQUESTED\" }")

// setup returns a synced F6 client whose upstream is a real connector, and the proxy and connector captures.
func setup(t *testing.T, at string) (*testkit.Client, *httptest.Server, *capture, *capture) {
	t.Helper()
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(s, testkit.F6Routes())
	s.Clock.Set(at)
	if err := c.Sync.Run(); err != nil {
		t.Fatal(err)
	}
	atConnector := &capture{}
	conn := httptest.NewServer(atConnector.wrap(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("X-Connector", "1")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write(connectorBody)
	})))
	t.Cleanup(conn.Close)
	up, _ := proxy_dsp.NewUpstream(conn.URL+"/protocol", "")
	c.Edge.Upstream = up
	atProxy := &capture{}
	proxy := httptest.NewServer(atProxy.wrap(proxy_dsp.Handler{Edge: c.Edge, Listener: domain.DSPIngress}))
	t.Cleanup(proxy.Close)
	return c, proxy, atProxy, atConnector
}

// post sends a body with custom headers and returns the response and its body.
func post(t *testing.T, base, path string, body []byte) (*http.Response, []byte) {
	t.Helper()
	req, _ := http.NewRequest("POST", base+path, bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer counterparty-token")
	req.Header.Set("Content-Type", "application/json")
	req.Header["x-odd-CASE"] = []string{"kept"}
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

func TestT_SYS_003_RequestForwardedByteForByte(t *testing.T) {
	_, proxy, atProxy, atConn := setup(t, "2026-10-10T12:00:00Z")
	body := append(testkit.ContractRequest("odrl:use"), []byte("\n  ")...)
	post(t, proxy.URL, "/negotiations/request", body)
	if !bytes.Equal(atConn.body, body) || !reflect.DeepEqual(atConn.header, atProxy.header) || atConn.host != atProxy.host || atConn.path != "/protocol/negotiations/request" {
		t.Fatalf("request changed:\nproxy %v %q\nconn  %v %q", atProxy.header, atProxy.host, atConn.header, atConn.host)
	}
}

func TestT_CLI_014_ResponseUnchangedExceptVerdict(t *testing.T) {
	_, proxy, _, _ := setup(t, "2026-10-10T12:00:00Z")
	resp, b := post(t, proxy.URL, "/negotiations/request", testkit.ContractRequest("odrl:use"))
	if !bytes.Equal(b, connectorBody) || resp.StatusCode != 201 || resp.Header.Get("X-Connector") != "1" || resp.Header.Get(domain.HeaderName) != "" {
		t.Fatalf("forward changed the response: %d %v %q", resp.StatusCode, resp.Header, b)
	}
	resp, b = post(t, proxy.URL, "/negotiations/request", testkit.ContractRequest("ex:access"))
	if !bytes.Equal(b, connectorBody) || resp.Header.Get(domain.HeaderName) == "" {
		t.Fatalf("forward-notice: %v %q", resp.Header, b)
	}
}

func TestT_CLI_019_AdminNotServedOnDSP(t *testing.T) {
	_, proxy, _, atConn := setup(t, "2026-10-10T12:00:00Z")
	resp, err := http.Get(proxy.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if atConn.path != "/protocol/status" || resp.StatusCode != http.StatusCreated {
		t.Fatalf("/status on dsp-ingress did not reach the connector: %s %d", atConn.path, resp.StatusCode)
	}
}

func TestEgressForwardProxyCapturesProviderAgreement(t *testing.T) {
	c, _, _, atConn := setup(t, "2026-10-10T12:00:00Z")
	egress := httptest.NewServer(proxy_dsp.Handler{Edge: c.Edge, Listener: domain.DSPEgress})
	defer egress.Close()
	pu, _ := url.Parse(egress.URL)
	cl := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(pu)}}
	conn, _ := url.Parse(c.Edge.Upstream.(*proxy_dsp.Upstream).DSP.String())
	target := conn.Scheme + "://" + conn.Host + "/negotiations/c-1/agreement"
	resp, err := cl.Post(target, "application/json", bytes.NewReader(testkit.AgreementMessage("urn:uuid:A1", "ex:access", "")))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	e, ok, _ := c.Repo.Get("urn:uuid:A1")
	if atConn.path != "/negotiations/c-1/agreement" || !ok || e.Role != "provider" || e.Version != "1.0.0" {
		t.Fatalf("egress: path %s entry %+v", atConn.path, e)
	}
}
