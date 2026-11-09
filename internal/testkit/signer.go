// Package testkit provides an in-memory Ed25519 signer with key rotation, implementing the server Signer port for tests.
// Rotation publishes a key before activating it, as REQ-HUB-015 requires.
package testkit

import (
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"sync"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/sign"
)

// signerKey is one key of the rotating signer.
type signerKey struct {
	pub       did.PublicKey
	priv      ed25519.PrivateKey
	published bool
}

// KeySigner holds several keys and signs with the active one.
type KeySigner struct {
	mu     sync.Mutex
	did    did.DID
	keys   []signerKey
	active int
	Fail   error
}

// NewKeySigner creates a signer with one published and active key #key-1.
func NewKeySigner(d did.DID) *KeySigner {
	s := &KeySigner{did: d}
	s.add(true)
	return s
}

// add generates a key and returns its kid.
func (s *KeySigner) add(published bool) string {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kid := fmt.Sprintf("%s#key-%d", s.did, len(s.keys)+1)
	s.keys = append(s.keys, signerKey{did.PublicKey{Kid: kid, Key: pub}, priv, published})
	return kid
}

// AddUnpublished creates a key that is not in the DID document.
func (s *KeySigner) AddUnpublished() string { s.mu.Lock(); defer s.mu.Unlock(); return s.add(false) }

// Publish adds kid to the DID document.
func (s *KeySigner) Publish(kid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.keys {
		if s.keys[i].pub.Kid == kid {
			s.keys[i].published = true
		}
	}
}

// Retire removes kid from the DID document.
func (s *KeySigner) Retire(kid string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.keys {
		if s.keys[i].pub.Kid == kid {
			s.keys[i].published = false
		}
	}
}

// Activate makes kid the signing key; it must already be published.
func (s *KeySigner) Activate(kid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, k := range s.keys {
		if k.pub.Kid == kid {
			if !k.published {
				return errors.New("testkit: key must be published before it signs")
			}
			s.active = i
			return nil
		}
	}
	return errors.New("testkit: unknown kid")
}

// PublishedKeys returns the keys of the DID document.
func (s *KeySigner) PublishedKeys() []did.PublicKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []did.PublicKey
	for _, k := range s.keys {
		if k.published {
			out = append(out, k.pub)
		}
	}
	return out
}

// Sign signs msg with the active key, refusing if it is not published.
func (s *KeySigner) Sign(msg []byte) (sign.Signature, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Fail != nil {
		return sign.Signature{}, s.Fail
	}
	k := s.keys[s.active]
	if !k.published {
		return sign.Signature{}, errors.New("testkit: active key is not published")
	}
	return sign.Signature{Kid: k.pub.Kid, Alg: sign.AlgEdDSA, Sig: sign.Encode(ed25519.Sign(k.priv, msg))}, nil
}

// Document returns the current DID document.
func (s *KeySigner) Document() did.Document { return did.NewDocument(s.did, s.PublishedKeys()) }

// SignWith signs msg with kid even if it is no longer published (to forge stale or retired signatures).
func (s *KeySigner) SignWith(kid string, msg []byte) sign.Signature {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, k := range s.keys {
		if k.pub.Kid == kid {
			return sign.Signature{Kid: kid, Alg: sign.AlgEdDSA, Sig: sign.Encode(ed25519.Sign(k.priv, msg))}
		}
	}
	return sign.Signature{}
}
