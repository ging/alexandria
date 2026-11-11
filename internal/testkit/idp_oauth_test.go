// Tests of the interactive fake IdP: discovery, request validation, authorization code with PKCE, single use, refresh and CORS.
// They drive the handler in-process and decode issued tokens without verifying signatures (the hub adapter tests cover that).
package testkit

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// issuer is the issuer URL of the server under test.
const issuer = "http://127.0.0.1:9400"

// verifier and callback are the PKCE verifier and redirect URI of the test client.
const (
	verifier = "test-verifier-0123456789-0123456789-0123456789"
	callback = "http://localhost:5273/auth/callback"
)

// fixed is the frozen instant used to issue tokens.
var fixed = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

// newIDPServer returns a handler with a fresh key and a frozen clock.
func newIDPServer() http.Handler {
	return NewOAuth(NewIDP(issuer), func() time.Time { return fixed }, time.Hour).Handler()
}

// call performs a request with an optional form body and returns the recorder.
func call(h http.Handler, method, target string, form url.Values) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequestWithContext(context.Background(), method, target, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// request builds valid authorization parameters, overridden by extra.
func request(extra map[string]string) url.Values {
	sum := sha256.Sum256([]byte(verifier))
	v := url.Values{
		"response_type": {"code"}, "client_id": {"console"}, "redirect_uri": {callback}, "state": {"s1"}, "nonce": {"n1"},
		"code_challenge": {base64.RawURLEncoding.EncodeToString(sum[:])}, "code_challenge_method": {"S256"},
	}
	for k, val := range extra {
		v.Set(k, val)
	}
	return v
}

// signIn completes the form as sub and returns the issued authorization code.
func signIn(t *testing.T, h http.Handler, sub string) string {
	t.Helper()
	rec := call(h, http.MethodPost, "/authorize", request(map[string]string{"sub": sub}))
	loc, err := url.Parse(rec.Header().Get("Location"))
	if rec.Code != http.StatusSeeOther || err != nil || loc.Query().Get("state") != "s1" {
		t.Fatalf("status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
	return loc.Query().Get("code")
}

// redeem exchanges a code with a verifier and returns the status and decoded body.
func redeem(h http.Handler, code, v string) (int, map[string]any) {
	rec := call(h, http.MethodPost, "/token", url.Values{"grant_type": {"authorization_code"}, "code": {code}, "code_verifier": {v}, "redirect_uri": {callback}, "client_id": {"console"}})
	var out map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

// claims decodes the payload of a compact JWT without verifying it.
func claims(t *testing.T, token string) map[string]any {
	t.Helper()
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("not a JWT: %q", token)
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	_ = json.Unmarshal(b, &out)
	return out
}

// audience returns the aud claim whether it is serialized as a string or a one-element list.
func audience(c map[string]any) string {
	if list, ok := c["aud"].([]any); ok && len(list) > 0 {
		s, _ := list[0].(string)
		return s
	}
	s, _ := c["aud"].(string)
	return s
}

func TestDiscoveryAdvertisesInteractiveEndpoints(t *testing.T) {
	var doc map[string]any
	_ = json.Unmarshal(call(newIDPServer(), http.MethodGet, "/.well-known/openid-configuration", nil).Body.Bytes(), &doc)
	if doc["authorization_endpoint"] != issuer+"/authorize" || doc["token_endpoint"] != issuer+"/token" || doc["jwks_uri"] != issuer+"/jwks.json" {
		t.Fatalf("metadata %v", doc)
	}
}

func TestResponsesAllowCrossOriginBrowsers(t *testing.T) {
	for _, target := range []string{"/.well-known/openid-configuration", "/jwks.json"} {
		if got := call(newIDPServer(), http.MethodGet, target, nil).Header().Get("Access-Control-Allow-Origin"); got != "*" {
			t.Errorf("%s: Access-Control-Allow-Origin = %q", target, got)
		}
	}
}

func TestPreflightIsAnswered(t *testing.T) {
	if rec := call(newIDPServer(), http.MethodOptions, "/token", nil); rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAuthorizePageRendersForValidRequest(t *testing.T) {
	rec := call(newIDPServer(), http.MethodGet, "/authorize?"+request(nil).Encode(), nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `name="code_challenge"`) {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestAuthorizeRejectsNonLoopbackRedirect(t *testing.T) {
	rec := call(newIDPServer(), http.MethodPost, "/authorize", request(map[string]string{"sub": "ana", "redirect_uri": "https://evil.example/cb"}))
	if rec.Code != http.StatusBadRequest || rec.Header().Get("Location") != "" {
		t.Fatalf("status %d location %q", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuthorizeRequiresPKCE(t *testing.T) {
	if rec := call(newIDPServer(), http.MethodPost, "/authorize", request(map[string]string{"sub": "ana", "code_challenge": ""})); rec.Code != http.StatusBadRequest {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestCodeFlowIssuesAccessTokenForSubject(t *testing.T) {
	h := newIDPServer()
	status, body := redeem(h, signIn(t, h, "sergio"), verifier)
	if status != http.StatusOK {
		t.Fatalf("status %d body %v", status, body)
	}
	c := claims(t, body["access_token"].(string))
	if c["sub"] != "sergio" || c["iss"] != issuer || audience(c) != "alexandria-vhub" {
		t.Fatalf("claims %v", c)
	}
}

func TestPresetSignsInAsAdmin(t *testing.T) {
	h := newIDPServer()
	rec := call(h, http.MethodPost, "/authorize", request(map[string]string{"preset": "ana:admin"}))
	loc, _ := url.Parse(rec.Header().Get("Location"))
	_, body := redeem(h, loc.Query().Get("code"), verifier)
	c := claims(t, body["access_token"].(string))
	if roles, _ := c["roles"].([]any); c["sub"] != "ana" || len(roles) != 1 || roles[0] != "vhub-admin" {
		t.Fatalf("claims %v", c)
	}
}

func TestIDTokenCarriesNonceAndClientAudience(t *testing.T) {
	h := newIDPServer()
	_, body := redeem(h, signIn(t, h, "ana"), verifier)
	c := claims(t, body["id_token"].(string))
	if c["nonce"] != "n1" || audience(c) != "console" {
		t.Fatalf("claims %v", c)
	}
}

func TestWrongVerifierIsRejected(t *testing.T) {
	h := newIDPServer()
	if status, body := redeem(h, signIn(t, h, "ana"), "another-verifier"); status != http.StatusBadRequest || body["error"] != "invalid_grant" {
		t.Fatalf("status %d body %v", status, body)
	}
}

func TestCodeIsSingleUse(t *testing.T) {
	h := newIDPServer()
	code := signIn(t, h, "ana")
	_, _ = redeem(h, code, verifier)
	if status, _ := redeem(h, code, verifier); status != http.StatusBadRequest {
		t.Fatalf("status %d", status)
	}
}

func TestRefreshTokenRotates(t *testing.T) {
	h := newIDPServer()
	_, first := redeem(h, signIn(t, h, "sergio"), verifier)
	form := url.Values{"grant_type": {"refresh_token"}, "refresh_token": {first["refresh_token"].(string)}, "client_id": {"console"}}
	rec := call(h, http.MethodPost, "/token", form)
	var second map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &second)
	if rec.Code != http.StatusOK || second["refresh_token"] == first["refresh_token"] || claims(t, second["access_token"].(string))["sub"] != "sergio" {
		t.Fatalf("status %d body %v", rec.Code, second)
	}
	if again := call(h, http.MethodPost, "/token", form); again.Code != http.StatusBadRequest {
		t.Fatalf("reused refresh token: status %d", again.Code)
	}
}
