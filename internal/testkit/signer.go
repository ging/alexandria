// Package testkit provides the hub key doubles: an Ed25519 signer with did:key or did:web identity and a switchable DID document.
// Tests publish, retire and rotate keys to exercise REQ-HUB-015, FL9 and V0–V4 (DP-5, REQ-CLI-024, REQ-CLI-027).
package testkit

import (
	"crypto/ed25519"
	"errors"
	"sync"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
)

// KeySigner signs with a deterministic Ed25519 key; it implements app.KeySigner.
type KeySigner struct {
	mu   sync.Mutex
	priv ed25519.PrivateKey
	vm   string
}

// NewKeySigner returns a signer from a seed byte; vm is the verification method, or the did:key method when empty.
func NewKeySigner(seed byte, vm string) *KeySigner {
	s := make([]byte, ed25519.SeedSize)
	s[0] = seed
	priv := ed25519.NewKeyFromSeed(s)
	if vm == "" {
		_, vm = did.KeyDID(priv.Public().(ed25519.PublicKey))
	}
	return &KeySigner{priv: priv, vm: vm}
}

// KeyDIDOf returns the did:key of the signer key.
func (s *KeySigner) KeyDIDOf() did.DID { d, _ := did.KeyDID(s.PublicKey()); return d }

// VerificationMethod returns the active verification method.
func (s *KeySigner) VerificationMethod() string { s.mu.Lock(); defer s.mu.Unlock(); return s.vm }

// PublicKey returns the active public key.
func (s *KeySigner) PublicKey() ed25519.PublicKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.priv.Public().(ed25519.PublicKey)
}

// Sign adds an eddsa-jcs-2022 proof created at the given instant.
func (s *KeySigner) Sign(unsecured []byte, created timex.Instant) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return vc.Sign(unsecured, vc.Options(s.vm, created), s.priv)
}

// Rotate switches to a new key and verification method.
func (s *KeySigner) Rotate(seed byte, vm string) {
	n := NewKeySigner(seed, vm)
	s.mu.Lock()
	s.priv, s.vm = n.priv, n.vm
	s.mu.Unlock()
}

// Resolver resolves did:key locally and did:web from a settable document, counting resolutions.
type Resolver struct {
	mu    sync.Mutex
	docs  map[did.DID]did.Document
	Fail  bool
	Calls int
}

// NewResolver returns a resolver without did:web documents.
func NewResolver() *Resolver { return &Resolver{docs: map[did.DID]did.Document{}} }

// SetDocument publishes a did:web document.
func (r *Resolver) SetDocument(d did.DID, keys ...did.PublicKey) {
	r.mu.Lock()
	r.docs[d] = did.NewDocument(d, keys)
	r.mu.Unlock()
}

// Resolve returns the document of d.
func (r *Resolver) Resolve(d did.DID) (did.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Calls++
	if d.Method() == "key" {
		return did.ResolveKey(d)
	}
	doc, ok := r.docs[d]
	if r.Fail || !ok {
		return did.Document{}, errors.New("testkit: DID document unavailable")
	}
	return doc, nil
}
