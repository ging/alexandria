// Package testkit provides the fake IdP: an ES256 key, OIDC discovery and JWKS handlers, and at+jwt access tokens with a roles claim.
// deploy/fake-idp serves the same logic, so the hub validates real tokens with its jwt_jwks adapter (DP-4, PRE-07, REQ-HUB-022).
package testkit

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
)

// IdP holds the signing key and issuer of the fake identity provider.
type IdP struct {
	Issuer string
	Key    *ecdsa.PrivateKey
	Kid    string
}

// NewIdP returns an IdP with a fresh P-256 key.
func NewIdP(issuer string) *IdP {
	k, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return &IdP{Issuer: issuer, Key: k, Kid: "idp-1"}
}

// LoadIdP reads a P-256 key from a PEM file, generating it if absent.
func LoadIdP(issuer, path string) (*IdP, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- test and demo key path.
	if errors.Is(err, os.ErrNotExist) {
		idp := NewIdP(issuer)
		der, _ := x509.MarshalPKCS8PrivateKey(idp.Key)
		return idp, os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600)
	}
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("idp: no PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	ek, ok := k.(*ecdsa.PrivateKey)
	if !ok {
		return nil, errors.New("idp: key is not ECDSA")
	}
	return &IdP{Issuer: issuer, Key: ek, Kid: "idp-1"}, nil
}

// Handler serves /.well-known/openid-configuration and /jwks.json.
func (i *IdP) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"issuer": i.Issuer, "jwks_uri": i.Issuer + "/jwks.json", "id_token_signing_alg_values_supported": []string{"ES256"}})
	})
	mux.HandleFunc("GET /jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &i.Key.PublicKey, KeyID: i.Kid, Algorithm: "ES256", Use: "sig"}}})
	})
	return mux
}

// Token describes an access token to mint; zero values take the defaults.
type Token struct {
	Sub      string
	Admin    bool
	Audience string
	Issuer   string
	Typ      string
	Kid      string
	Now      time.Time
	TTL      time.Duration
	NotYet   bool
	ForeignK bool
}

// Mint signs an at+jwt access token (RFC 9068) with the roles claim.
func (i *IdP) Mint(t Token) (string, error) {
	if t.Typ == "" {
		t.Typ = "at+jwt"
	}
	if t.Kid == "" {
		t.Kid = i.Kid
	}
	if t.Issuer == "" {
		t.Issuer = i.Issuer
	}
	if t.Audience == "" {
		t.Audience = "alexandria-vhub"
	}
	if t.TTL == 0 {
		t.TTL = time.Hour
	}
	key := i.Key
	if t.ForeignK {
		key, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	}
	opts := (&jose.SignerOptions{}).WithType(jose.ContentType(t.Typ)).WithHeader("kid", t.Kid)
	sig, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, opts)
	if err != nil {
		return "", err
	}
	roles := []string{}
	if t.Admin {
		roles = append(roles, "vhub-admin")
	}
	nbf := t.Now
	if t.NotYet {
		nbf = t.Now.Add(time.Hour)
	}
	claims := jwt.Claims{Issuer: t.Issuer, Subject: t.Sub, Audience: jwt.Audience{t.Audience}, IssuedAt: jwt.NewNumericDate(t.Now), NotBefore: jwt.NewNumericDate(nbf), Expiry: jwt.NewNumericDate(t.Now.Add(t.TTL))}
	return jwt.Signed(sig).Claims(claims).Claims(map[string]any{"roles": roles, "client_id": "fake"}).Serialize()
}
