// Package signer_file implements the file signer: one Ed25519 hub key read from a PKCS#8 PEM file, generated on first start (PoC only).
// Implements the Signer port; it only signs with the key it publishes in the DID document (DP-4, REQ-HUB-010, REQ-HUB-015).
package signer_file

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/server/app"
)

// Signer holds the single hub key (R10).
type Signer struct {
	key ed25519.PrivateKey
	pub did.PublicKey
}

var _ app.Signer = (*Signer)(nil)

// Load reads the key at path, generating and saving a new one if the file does not exist.
func Load(path string, d did.DID) (*Signer, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator in server.json.
	if errors.Is(err, os.ErrNotExist) {
		return generate(path, d)
	}
	if err != nil {
		return nil, err
	}
	blk, _ := pem.Decode(b)
	if blk == nil {
		return nil, errors.New("signer_file: no PEM block")
	}
	k, err := x509.ParsePKCS8PrivateKey(blk.Bytes)
	if err != nil {
		return nil, err
	}
	priv, ok := k.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("signer_file: key is not Ed25519")
	}
	return newSigner(priv, d), nil
}

// generate creates and stores a fresh key with owner-only permissions.
func generate(path string, d did.DID) (*Signer, error) {
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	der, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), 0o600); err != nil {
		return nil, err
	}
	return newSigner(priv, d), nil
}

// newSigner derives the published key with kid <did>#key-1.
func newSigner(priv ed25519.PrivateKey, d did.DID) *Signer {
	return &Signer{key: priv, pub: did.PublicKey{Kid: string(d) + "#key-1", Key: priv.Public().(ed25519.PublicKey)}}
}

// PublishedKeys returns the key listed in the DID document.
func (s *Signer) PublishedKeys() []did.PublicKey { return []did.PublicKey{s.pub} }

// Sign signs msg with the published key.
func (s *Signer) Sign(msg []byte) (sign.Signature, error) {
	return sign.Signature{Kid: s.pub.Kid, Alg: sign.AlgEdDSA, Sig: sign.Encode(ed25519.Sign(s.key, msg))}, nil
}
