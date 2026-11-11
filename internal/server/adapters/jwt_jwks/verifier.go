// Package jwt_jwks validates OAuth 2.0 JWT access tokens (RFC 9068) with checks A1–A5 in order, fetching each issuer's JWKS by OIDC Discovery.
// It produces the (iss, sub) principal and the admin flag from the configured claim; an unknown kid triggers one JWKS refresh (§3.5, REQ-HUB-022).
package jwt_jwks

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/domain"
)

// Issuer is one accepted token issuer and its discovery URL.
type Issuer struct {
	Iss          string
	DiscoveryURL string
}

// Verifier implements the TokenVerifier of the /v1 adapter.
type Verifier struct {
	Issuers    []Issuer
	Audience   string
	Algorithms []string
	AdminClaim string
	AdminValue string
	Skew       timex.Duration
	CacheTTL   timex.Duration
	Clock      timex.Clock
	Client     *http.Client

	mu    sync.Mutex
	cache map[string]cached
}

// cached is a JWKS with the hub instant it was fetched.
type cached struct {
	set jose.JSONWebKeySet
	at  timex.Instant
}

// Failure names the first check A1–A5 that failed.
type Failure struct {
	Check string
	Err   error
}

// Error implements error.
func (f *Failure) Error() string { return f.Check + ": " + f.Err.Error() }

// fail builds a Failure.
func fail(check, format string, args ...any) error {
	return &Failure{Check: check, Err: fmt.Errorf(format, args...)}
}

// Verify applies A1–A5 to a bearer token and returns its principal.
func (v *Verifier) Verify(raw string) (domain.Principal, error) {
	algs := make([]jose.SignatureAlgorithm, 0, len(v.Algorithms))
	for _, a := range v.Algorithms {
		algs = append(algs, jose.SignatureAlgorithm(a))
	}
	tok, err := jwt.ParseSigned(raw, algs)
	if err != nil || len(tok.Headers) != 1 {
		return domain.Principal{}, fail("A1", "not a JWT with an allowed asymmetric alg: %v", err)
	}
	h := tok.Headers[0]
	if typ, _ := h.ExtraHeaders[jose.HeaderType].(string); !strings.EqualFold(typ, "at+jwt") && !strings.EqualFold(typ, "application/at+jwt") {
		return domain.Principal{}, fail("A1", "typ is %q, not at+jwt", typ)
	}
	var unsafe jwt.Claims
	if err := tok.UnsafeClaimsWithoutVerification(&unsafe); err != nil {
		return domain.Principal{}, fail("A2", "%v", err)
	}
	iss, ok := v.issuer(unsafe.Issuer)
	if !ok {
		return domain.Principal{}, fail("A2", "issuer %q is not configured", unsafe.Issuer)
	}
	var claims jwt.Claims
	extra := map[string]any{}
	key, err := v.key(iss, h.KeyID)
	if err != nil {
		return domain.Principal{}, fail("A3", "%v", err)
	}
	if err := tok.Claims(key, &claims, &extra); err != nil {
		return domain.Principal{}, fail("A3", "signature does not verify: %v", err)
	}
	if !slices.Contains([]string(claims.Audience), v.Audience) {
		return domain.Principal{}, fail("A4", "aud %v does not contain %s", claims.Audience, v.Audience)
	}
	now := v.Clock.Now()
	if claims.Expiry == nil || !now.Add(-v.Skew).Before(timex.FromTime(claims.Expiry.Time())) {
		return domain.Principal{}, fail("A5", "token expired")
	}
	if claims.NotBefore != nil && now.Add(v.Skew).Before(timex.FromTime(claims.NotBefore.Time())) {
		return domain.Principal{}, fail("A5", "token not yet valid")
	}
	return domain.Principal{Iss: claims.Issuer, Sub: claims.Subject, IsAdmin: hasValue(extra[v.AdminClaim], v.AdminValue)}, nil
}

// issuer returns the configured issuer named iss.
func (v *Verifier) issuer(iss string) (Issuer, bool) {
	for _, i := range v.Issuers {
		if i.Iss == iss {
			return i, true
		}
	}
	return Issuer{}, false
}

// hasValue reports whether a claim equals value or, as a list, contains it.
func hasValue(claim any, value string) bool {
	switch c := claim.(type) {
	case string:
		return c == value
	case []any:
		for _, x := range c {
			if x == value {
				return true
			}
		}
	}
	return false
}

// key returns the JWKS key with kid, refreshing the JWKS once when kid is unknown (A3).
func (v *Verifier) key(iss Issuer, kid string) (*jose.JSONWebKey, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.cache == nil {
		v.cache = map[string]cached{}
	}
	c, ok := v.cache[iss.Iss]
	stale := !ok || v.Clock.Now().Sub(c.at) >= v.CacheTTL
	for attempt := 0; attempt < 2; attempt++ {
		if stale {
			set, err := v.fetch(iss)
			if err != nil {
				return nil, err
			}
			c = cached{set: set, at: v.Clock.Now()}
			v.cache[iss.Iss] = c
		}
		if ks := c.set.Key(kid); len(ks) > 0 {
			return &ks[0], nil
		}
		if stale {
			break
		}
		stale = true
	}
	return nil, fmt.Errorf("kid %q not in the JWKS of %s", kid, iss.Iss)
}

// fetch downloads the JWKS through OIDC Discovery.
func (v *Verifier) fetch(iss Issuer) (jose.JSONWebKeySet, error) {
	c := v.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	var conf struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := getJSON(c, iss.DiscoveryURL, &conf); err != nil {
		return jose.JSONWebKeySet{}, err
	}
	if conf.Issuer != iss.Iss || conf.JWKSURI == "" {
		return jose.JSONWebKeySet{}, errors.New("discovery document does not match the issuer")
	}
	var set jose.JSONWebKeySet
	return set, getJSON(c, conf.JWKSURI, &set)
}

// getJSON GETs a JSON document.
func getJSON(c *http.Client, u string, v any) error {
	resp, err := c.Get(u) // #nosec G107 -- discovery and JWKS URLs come from server.json.
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: HTTP %d", u, resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}
