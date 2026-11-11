// Package testkit provides an HTTP hub on httptest: the in-memory hub behind the real /v1 and /pub adapters and the real jwt_jwks verifier.
// The fake IdP issues the tokens of Ana, Sergio and Eve, so A1–A5 and the §3.5 table run end to end (DP-4, REQ-HUB-017, REQ-HUB-022).
package testkit

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/internal/server/adapters/http_v1"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/adapters/jwt_jwks"
)

// HTTPServer is an F6 hub reachable over HTTP with valid tokens for its principals.
type HTTPServer struct {
	*Server
	URL                       string
	IdP                       *IdP
	AnaTok, SergioTok, EveTok string
	server                    *httptest.Server
}

// NewHTTPServer starts an F6 hub at t0 with namespace demo, behind httptest.
func NewHTTPServer(t *testing.T, t0 string) *HTTPServer {
	t.Helper()
	var idp *IdP
	idpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { idp.Handler().ServeHTTP(w, r) }))
	t.Cleanup(idpSrv.Close)
	idp = NewIdP(idpSrv.URL)
	s := NewBareServer(t0)
	h := &HTTPServer{Server: s, IdP: idp}
	verifier := &jwt_jwks.Verifier{
		Issuers: []jwt_jwks.Issuer{{Iss: idp.Issuer, DiscoveryURL: idp.Issuer + "/.well-known/openid-configuration"}}, Audience: "alexandria-vhub",
		Algorithms: []string{"ES256"}, AdminClaim: "roles", AdminValue: "vhub-admin", Skew: 5 * timex.Second, CacheTTL: 10 * timex.Minute, Clock: s.Clock,
	}
	mux := http.NewServeMux()
	http_v1.Handler{Hub: s.Hub, Tokens: verifier, Context: Context, Base: Base}.Register(mux)
	http_pub.Handler{Pub: s.Hub.Publication, Problems: httpx.Problems{Context: Context}}.Register(mux)
	h.server = httptest.NewServer(httpx.WithDate(s.Clock, mux))
	t.Cleanup(h.server.Close)
	h.URL = h.server.URL
	now := s.Clock.Now().Time()
	mint := func(sub string, admin bool) string {
		tok, err := idp.Mint(Token{Sub: sub, Admin: admin, Now: now, TTL: 365 * 24 * time.Hour})
		if err != nil {
			t.Fatal(err)
		}
		return tok
	}
	h.AnaTok, h.SergioTok, h.EveTok = mint("ana", true), mint("sergio", false), mint("eve", false)
	Ana.Iss, Sergio.Iss, Eve.Iss = idp.Issuer, idp.Issuer, idp.Issuer
	return h
}

// Do sends a request with an optional bearer token and headers and returns status, headers and body.
func (h *HTTPServer) Do(t *testing.T, method, path, token string, body []byte, hdr map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, h.URL+path, bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

// SetupHTTP creates namespace demo and assigns Sergio through the API (F6.1 steps 1–2).
func (h *HTTPServer) SetupHTTP(t *testing.T) {
	t.Helper()
	if st, _, b := h.Do(t, "POST", "/v1/namespaces", h.AnaTok, []byte(`{"slug":"demo"}`), nil); st != 201 {
		t.Fatalf("create namespace: %d %s", st, b)
	}
	if st, _, b := h.Do(t, "POST", "/v1/namespaces/demo/stewards", h.AnaTok, []byte(`{"iss":"`+h.IdP.Issuer+`","sub":"sergio"}`), nil); st != 201 {
		t.Fatalf("assign steward: %d %s", st, b)
	}
}
