// Package did defines DIDs, DID documents with Multikey methods, assertionMethod lookup and the local did:key resolution.
// Shared by the hub (checks its own key, REQ-HUB-015) and the client (verifies hub proofs, REQ-CLI-024, REQ-SYS-009).
package did

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/mr-tron/base58"
)

// DID is a decentralized identifier of method did:web or did:key (Q14).
type DID string

// Method returns "web", "key" or an empty string.
func (d DID) Method() string {
	switch {
	case strings.HasPrefix(string(d), "did:web:"):
		return "web"
	case strings.HasPrefix(string(d), "did:key:z"):
		return "key"
	}
	return ""
}

// OfURL returns the DID part of a DID URL (before '#').
func OfURL(vm string) DID { d, _, _ := strings.Cut(vm, "#"); return DID(d) }

// PublicKey is an Ed25519 verification key identified by its verification method DID URL.
type PublicKey struct {
	Kid string
	Key ed25519.PublicKey
}

// Multicodec prefixes of Ed25519 public and private keys.
var (
	edPub  = []byte{0xed, 0x01}
	edPriv = []byte{0x80, 0x26}
)

// EncodeMultikey renders an Ed25519 public key as publicKeyMultibase (z + base58btc of 0xed01 || key).
func EncodeMultikey(k ed25519.PublicKey) string {
	return "z" + base58.Encode(append(append([]byte{}, edPub...), k...))
}

// DecodeMultikey parses an Ed25519 publicKeyMultibase.
func DecodeMultikey(s string) (ed25519.PublicKey, error) {
	b, err := decodeZ(s, edPub, ed25519.PublicKeySize)
	return ed25519.PublicKey(b), err
}

// DecodePrivateMultibase parses an Ed25519 secretKeyMultibase (0x8026 || seed) into a private key.
func DecodePrivateMultibase(s string) (ed25519.PrivateKey, error) {
	seed, err := decodeZ(s, edPriv, ed25519.SeedSize)
	if err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

// decodeZ decodes a base58btc multibase value with the given multicodec prefix and payload size.
func decodeZ(s string, prefix []byte, size int) ([]byte, error) {
	if !strings.HasPrefix(s, "z") {
		return nil, errors.New("did: multibase value is not base58btc")
	}
	b, err := base58.Decode(s[1:])
	if err != nil || len(b) != len(prefix)+size || b[0] != prefix[0] || b[1] != prefix[1] {
		return nil, errors.New("did: not an Ed25519 multikey")
	}
	return b[len(prefix):], nil
}

// VerificationMethod is one entry of verificationMethod in a DID document.
type VerificationMethod struct {
	ID                 string `json:"id"`
	Type               string `json:"type"`
	Controller         string `json:"controller"`
	PublicKeyMultibase string `json:"publicKeyMultibase"`
}

// Document is the subset of a DID document used by vhub.
type Document struct {
	Context            []string             `json:"@context"`
	ID                 string               `json:"id"`
	VerificationMethod []VerificationMethod `json:"verificationMethod"`
	AssertionMethod    []string             `json:"assertionMethod"`
}

// NewDocument builds a DID document whose keys are all Multikey assertion methods.
func NewDocument(d DID, keys []PublicKey) Document {
	doc := Document{
		Context:            []string{"https://www.w3.org/ns/did/v1", "https://w3id.org/security/multikey/v1"},
		ID:                 string(d),
		VerificationMethod: []VerificationMethod{},
		AssertionMethod:    []string{},
	}
	for _, k := range keys {
		doc.VerificationMethod = append(doc.VerificationMethod, VerificationMethod{
			ID: k.Kid, Type: "Multikey", Controller: string(d), PublicKeyMultibase: EncodeMultikey(k.Key),
		})
		doc.AssertionMethod = append(doc.AssertionMethod, k.Kid)
	}
	return doc
}

// Parse decodes a DID document and makes relative method ids absolute.
func Parse(b []byte) (Document, error) {
	var doc Document
	if err := json.Unmarshal(b, &doc); err != nil {
		return Document{}, fmt.Errorf("did: %w", err)
	}
	abs := func(id string) string {
		if strings.HasPrefix(id, "#") {
			return doc.ID + id
		}
		return id
	}
	for i := range doc.VerificationMethod {
		doc.VerificationMethod[i].ID = abs(doc.VerificationMethod[i].ID)
	}
	for i := range doc.AssertionMethod {
		doc.AssertionMethod[i] = abs(doc.AssertionMethod[i])
	}
	return doc, nil
}

// AssertionKey returns the Ed25519 Multikey for kid if it is referenced by assertionMethod.
func (d Document) AssertionKey(kid string) (PublicKey, bool) {
	listed := false
	for _, a := range d.AssertionMethod {
		listed = listed || a == kid
	}
	if !listed {
		return PublicKey{}, false
	}
	for _, vm := range d.VerificationMethod {
		if vm.ID != kid || vm.Type != "Multikey" {
			continue
		}
		k, err := DecodeMultikey(vm.PublicKeyMultibase)
		if err != nil {
			return PublicKey{}, false
		}
		return PublicKey{Kid: kid, Key: k}, true
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

// KeyDID returns the did:key of an Ed25519 public key and its single verification method.
func KeyDID(k ed25519.PublicKey) (DID, string) {
	mb := EncodeMultikey(k)
	return DID("did:key:" + mb), "did:key:" + mb + "#" + mb
}

// ResolveKey computes the DID document of a did:key locally; it never touches the network.
func ResolveKey(d DID) (Document, error) {
	mb, ok := strings.CutPrefix(string(d), "did:key:")
	if !ok {
		return Document{}, fmt.Errorf("did: %q is not a did:key", d)
	}
	k, err := DecodeMultikey(mb)
	if err != nil {
		return Document{}, err
	}
	return NewDocument(d, []PublicKey{{Kid: string(d) + "#" + mb, Key: k}}), nil
}

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

// WebDID returns the did:web identifying a host[:port] (the port is percent-encoded).
func WebDID(hostport string) DID { return DID("did:web:" + strings.ReplaceAll(hostport, ":", "%3A")) }

// IsLocalhost reports whether a host[:port] designates the loopback interface.
func IsLocalhost(hostport string) bool {
	h := hostport
	if i := strings.LastIndex(hostport, ":"); i >= 0 && !strings.HasSuffix(hostport, "]") {
		h = hostport[:i]
	}
	return h == "localhost" || h == "127.0.0.1" || h == "[::1]"
}
