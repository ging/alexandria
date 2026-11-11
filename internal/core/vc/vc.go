// Package vc implements the common credential form of §3.8 and the eddsa-jcs-2022 Data Integrity cryptosuite (sign and verify).
// Verification works on the bytes as received and never resolves a JSON-LD context (R13, REQ-SYS-002, REQ-SYS-009, DP-3).
package vc

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/mr-tron/base58"
)

// Fixed values of §3.8.
const (
	CredentialsV2   = "https://www.w3.org/ns/credentials/v2"
	ProofType       = "DataIntegrityProof"
	Cryptosuite     = "eddsa-jcs-2022"
	AssertionMethod = "assertionMethod"
)

// Credential types of §3.8.
const (
	TypeVersionRecord   = "VersionRecord"
	TypeVersionListing  = "VersionListing"
	TypePackageManifest = "PackageManifest"
)

// Proof is a DataIntegrityProof; ProofValue is empty in proof options.
type Proof struct {
	Type               string `json:"type"`
	Cryptosuite        string `json:"cryptosuite"`
	Created            string `json:"created"`
	VerificationMethod string `json:"verificationMethod"`
	ProofPurpose       string `json:"proofPurpose"`
	ProofValue         string `json:"proofValue,omitempty"`
}

// Credential is the common form of §3.8 with a typed credentialSubject.
type Credential[T any] struct {
	Context    []string       `json:"@context"`
	Type       []string       `json:"type"`
	Issuer     string         `json:"issuer"`
	ValidFrom  timex.Instant  `json:"validFrom"`
	ValidUntil *timex.Instant `json:"validUntil,omitempty"`
	Subject    T              `json:"credentialSubject"`
	Proof      *Proof         `json:"proof,omitempty"`
}

// New builds an unsigned credential of type t with the contexts [credentials/v2, c].
func New[T any](t, c, issuer string, validFrom timex.Instant, validUntil *timex.Instant, subject T) Credential[T] {
	return Credential[T]{
		Context: []string{CredentialsV2, c}, Type: []string{"VerifiableCredential", t},
		Issuer: issuer, ValidFrom: validFrom, ValidUntil: validUntil, Subject: subject,
	}
}

// ErrInvalid reports a proof that does not verify.
var ErrInvalid = errors.New("vc: proof does not verify")

// Options returns the proof options of §3.8 for a credential issued at created.
func Options(vm string, created timex.Instant) Proof {
	return Proof{Type: ProofType, Cryptosuite: Cryptosuite, Created: created.String(), VerificationMethod: vm, ProofPurpose: AssertionMethod}
}

// Sign adds an eddsa-jcs-2022 proof to an unsecured document and returns the secured document in JCS form.
func Sign(unsecured []byte, opts Proof, priv ed25519.PrivateKey) ([]byte, error) {
	var doc map[string]any
	if err := json.Unmarshal(unsecured, &doc); err != nil {
		return nil, fmt.Errorf("vc: %w", err)
	}
	delete(doc, "proof")
	opts.ProofValue = ""
	data, err := hashData(doc, toMap(opts))
	if err != nil {
		return nil, err
	}
	opts.ProofValue = "z" + base58.Encode(ed25519.Sign(priv, data))
	doc["proof"] = toMap(opts)
	return canon.Marshal(doc)
}

// ParseProof extracts the proof of a secured document.
func ParseProof(secured []byte) (Proof, error) {
	var w struct {
		Proof *Proof `json:"proof"`
	}
	if err := json.Unmarshal(secured, &w); err != nil || w.Proof == nil {
		return Proof{}, errors.New("vc: document without proof")
	}
	return *w.Proof, nil
}

// Verify checks the eddsa-jcs-2022 proof of a secured document with the given public key.
func Verify(secured []byte, key ed25519.PublicKey) error {
	var doc map[string]any
	if err := json.Unmarshal(secured, &doc); err != nil {
		return fmt.Errorf("vc: %w", err)
	}
	proof, ok := doc["proof"].(map[string]any)
	if !ok {
		return ErrInvalid
	}
	pv, _ := proof["proofValue"].(string)
	if len(pv) < 2 || pv[0] != 'z' || proof["cryptosuite"] != Cryptosuite || proof["type"] != ProofType {
		return ErrInvalid
	}
	sig, err := base58.Decode(pv[1:])
	if err != nil {
		return ErrInvalid
	}
	delete(doc, "proof")
	config := map[string]any{}
	for k, v := range proof {
		if k != "proofValue" {
			config[k] = v
		}
	}
	if pc, ok := config["@context"]; ok && !contextPrefix(doc["@context"], pc) {
		return ErrInvalid
	}
	data, err := hashData(doc, config)
	if err != nil || !ed25519.Verify(key, data, sig) {
		return ErrInvalid
	}
	return nil
}

// hashData is SHA-256(JCS(proofConfig)) || SHA-256(JCS(document)); proofConfig inherits the document @context.
func hashData(doc, config map[string]any) ([]byte, error) {
	if c, ok := doc["@context"]; ok {
		if _, has := config["@context"]; !has {
			config["@context"] = c
		}
	}
	pc, err := canon.Marshal(config)
	if err != nil {
		return nil, err
	}
	dc, err := canon.Marshal(doc)
	if err != nil {
		return nil, err
	}
	a, b := sha256.Sum256(pc), sha256.Sum256(dc)
	return append(a[:], b[:]...), nil
}

// contextPrefix reports whether the document @context starts with every value of the proof @context.
func contextPrefix(doc, proof any) bool {
	d, _ := doc.([]any)
	p, _ := proof.([]any)
	return len(p) <= len(d) && slices.EqualFunc(d[:len(p)], p, func(a, b any) bool { return a == b })
}

// toMap turns proof options into a generic object, dropping an empty proofValue.
func toMap(p Proof) map[string]any {
	b, _ := json.Marshal(p)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	return m
}
