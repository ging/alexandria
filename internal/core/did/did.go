// Package did defines DID, DID URL and DID document types with Ed25519 key lookup in assertionMethod.
// Used by the hub to publish its key and by the client to verify signatures (REQ-SYS-009, REQ-HUB-010, REQ-CLI-024, DP-4, DP-5).
package did

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// DID is a decentralized identifier; v1 only supports did:web (Q14).
type DID string

// PublicKey is an Ed25519 verification key identified by a DID URL with fragment.
type PublicKey struct {
	Kid string
	Key ed25519.PublicKey
}

// JWK is the OKP/Ed25519 JSON Web Key form of a public key (RFC 8037).
type JWK struct {
	Kty string `json:"kty"`
	Crv string `json:"crv"`
	X   string `json:"x"`
}

// VerificationMethod is one entry of verificationMethod in a DID document.
type VerificationMethod struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	Controller   string `json:"controller"`
	PublicKeyJwk JWK    `json:"publicKeyJwk"`
}

// Document is the subset of a DID document used by vhub.
type Document struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	VerificationMethod []VerificationMethod `json:"verificationMethod"`
	AssertionMethod    []string             `json:"assertionMethod"`
}

// NewDocument builds the hub DID document from its published keys.
func NewDocument(d DID, keys []PublicKey) Document {
	doc := Document{
		Context:            []string{"https://www.w3.org/ns/did/v1", "https://w3id.org/security/suites/jws-2020/v1"},
		ID:                 string(d),
		VerificationMethod: []VerificationMethod{},
		AssertionMethod:    []string{},
	}
	for _, k := range keys {
		doc.VerificationMethod = append(doc.VerificationMethod, VerificationMethod{
			ID: k.Kid, Type: "JsonWebKey2020", Controller: string(d),
			PublicKeyJwk: JWK{Kty: "OKP", Crv: "Ed25519", X: base64.RawURLEncoding.EncodeToString(k.Key)},
		})
		doc.AssertionMethod = append(doc.AssertionMethod, k.Kid)
	}
	return doc
}

// AssertionKey returns the key for kid if it is referenced by assertionMethod.
func (d Document) AssertionKey(kid string) (PublicKey, bool) {
	listed := false
	for _, a := range d.AssertionMethod {
		if a == kid {
			listed = true
		}
	}
	if !listed {
		return PublicKey{}, false
	}
	for _, vm := range d.VerificationMethod {
		if vm.ID != kid || vm.PublicKeyJwk.Kty != "OKP" || vm.PublicKeyJwk.Crv != "Ed25519" {
			continue
		}
		raw, err := base64.RawURLEncoding.DecodeString(vm.PublicKeyJwk.X)
		if err != nil || len(raw) != ed25519.PublicKeySize {
			return PublicKey{}, false
		}
		return PublicKey{Kid: kid, Key: ed25519.PublicKey(raw)}, true
	}
	return PublicKey{}, false
}

// AssertionKeys returns every usable assertion key of the document.
func (d Document) AssertionKeys() []PublicKey {
	var out []PublicKey
	for _, a := range d.AssertionMethod {
		if k, ok := d.AssertionKey(a); ok {
			out = append(out, k)
		}
	}
	return out
}

// HasKid reports whether kid is an assertion key of the document.
func (d Document) HasKid(kid string) bool { _, ok := d.AssertionKey(kid); return ok }

// WebURL derives the DID document URL of a did:web; http is allowed only for localhost when asked (DP-5).
func WebURL(d DID, allowInsecureLocalhost bool) (string, error) {
	rest, ok := strings.CutPrefix(string(d), "did:web:")
	if !ok || rest == "" {
		return "", fmt.Errorf("did: %q is not a did:web", d)
	}
	parts := strings.Split(rest, ":")
	host, err := url.PathUnescape(parts[0])
	if err != nil || host == "" {
		return "", errors.New("did: invalid did:web host")
	}
	scheme := "https"
	if allowInsecureLocalhost && IsLocalhost(host) {
		scheme = "http"
	}
	if len(parts) == 1 {
		return scheme + "://" + host + "/.well-known/did.json", nil
	}
	return scheme + "://" + host + "/" + strings.Join(parts[1:], "/") + "/did.json", nil
}

// IsLocalhost reports whether a host[:port] designates the loopback interface.
func IsLocalhost(hostport string) bool {
	h := hostport
	if i := strings.LastIndex(hostport, ":"); i >= 0 && !strings.HasSuffix(hostport, "]") {
		h = hostport[:i]
	}
	return h == "localhost" || h == "127.0.0.1" || h == "[::1]"
}
