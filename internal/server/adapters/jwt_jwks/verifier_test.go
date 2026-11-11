// Tests A1–A5 with one token failing each check, the admin claim and the single JWKS refresh on an unknown kid.
// Fixtures: the testkit IdP served by httptest and a verifier on the hub fake clock (T-HUB-022, DP-4).
package jwt_jwks_test

import (
	"encoding/base64"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
	. "github.com/ging/alexandria/internal/server/adapters/jwt_jwks"
	"github.com/ging/alexandria/internal/testkit"
)

// now is the hub instant of the tests.
var now = time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)

// setup serves an IdP and returns it with a verifier and a JWKS request counter.
func setup(t *testing.T) (*testkit.IdP, *Verifier, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	var idp *testkit.IdP
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jwks.json" {
			hits.Add(1)
		}
		idp.Handler().ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	idp = testkit.NewIdP(srv.URL)
	v := &Verifier{
		Issuers: []Issuer{{Iss: srv.URL, DiscoveryURL: srv.URL + "/.well-known/openid-configuration"}}, Audience: "alexandria-vhub",
		Algorithms: []string{"ES256"}, AdminClaim: "roles", AdminValue: "vhub-admin", Skew: 5 * timex.Second, CacheTTL: 10 * timex.Minute,
		Clock: testkit.NewClock("2026-10-01T10:00:00Z"),
	}
	return idp, v, &hits
}

// check returns the failed check of err, or "".
func check(err error) string {
	var f *Failure
	if errors.As(err, &f) {
		return f.Check
	}
	return ""
}

func TestValidAdminToken(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "ana", Admin: true, Now: now})
	p, err := v.Verify(tok)
	if err != nil || p.Sub != "ana" || !p.IsAdmin {
		t.Fatalf("got %+v %v", p, err)
	}
}

func TestT_HUB_022_A1WrongTyp(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", Typ: "JWT", Now: now})
	if _, err := v.Verify(tok); check(err) != "A1" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A1AlgNone(t *testing.T) {
	_, v, _ := setup(t)
	enc := base64.RawURLEncoding.EncodeToString
	tok := enc([]byte(`{"alg":"none","typ":"at+jwt"}`)) + "." + enc([]byte(`{"sub":"x"}`)) + "."
	if _, err := v.Verify(tok); check(err) != "A1" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A2UnknownIssuer(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", Issuer: "https://evil.example", Now: now})
	if _, err := v.Verify(tok); check(err) != "A2" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A3ForeignKey(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", ForeignK: true, Now: now})
	if _, err := v.Verify(tok); check(err) != "A3" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A4WrongAudience(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", Audience: "other", Now: now})
	if _, err := v.Verify(tok); check(err) != "A4" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A5Expired(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", Now: now.Add(-2 * time.Hour)})
	if _, err := v.Verify(tok); check(err) != "A5" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_A5NotYetValid(t *testing.T) {
	idp, v, _ := setup(t)
	tok, _ := idp.Mint(testkit.Token{Sub: "x", NotYet: true, Now: now})
	if _, err := v.Verify(tok); check(err) != "A5" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_022_UnknownKidRefreshesOnce(t *testing.T) {
	idp, v, hits := setup(t)
	good, _ := idp.Mint(testkit.Token{Sub: "x", Now: now})
	_, _ = v.Verify(good)
	bad, _ := idp.Mint(testkit.Token{Sub: "x", Kid: "rotated", Now: now})
	if _, err := v.Verify(bad); check(err) != "A3" {
		t.Fatalf("got %v", err)
	}
	if n := hits.Load(); n != 2 {
		t.Fatalf("%d JWKS downloads, want 2", n)
	}
}
