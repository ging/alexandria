// Package signer_file implements the file signer: one Ed25519 hub key read from a PKCS#8 PEM file, generated on first start (PoC only).
// It signs eddsa-jcs-2022 proofs; the hub gates its use on the key being published in the DID document (DP-14, REQ-HUB-010, REQ-HUB-015).
package signer_file

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/app"
)

// Signer holds the single hub key (R10).
type Signer struct {
	key ed25519.PrivateKey
	vm  string
}

var _ app.KeySigner = (*Signer)(nil)

// Load reads the key at path, generating it if absent; vm defaults to the did:key method of the key.
func Load(path, vm string) (*Signer, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator in server.json.
	if errors.Is(err, os.ErrNotExist) {
		return generate(path, vm)
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
	return newSigner(priv, vm), nil
}

// generate creates and stores a fresh key with owner-only permissions.
func generate(path, vm string) (*Signer, error) {
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
	return newSigner(priv, vm), nil
}

// newSigner sets the verification method, defaulting to did:key.
func newSigner(priv ed25519.PrivateKey, vm string) *Signer {
	if vm == "" {
		_, vm = did.KeyDID(priv.Public().(ed25519.PublicKey))
	}
	return &Signer{key: priv, vm: vm}
}

// VerificationMethod returns the DID URL of the key.
func (s *Signer) VerificationMethod() string { return s.vm }

// PublicKey returns the public key.
func (s *Signer) PublicKey() ed25519.PublicKey { return s.key.Public().(ed25519.PublicKey) }

// KeyDID returns the did:key of the key (DP-5 default hub DID).
func (s *Signer) KeyDID() did.DID { d, _ := did.KeyDID(s.PublicKey()); return d }

// Sign adds an eddsa-jcs-2022 proof created at the given instant.
func (s *Signer) Sign(unsecured []byte, created timex.Instant) ([]byte, error) {
	return vc.Sign(unsecured, vc.Options(s.vm, created), s.key)
}
