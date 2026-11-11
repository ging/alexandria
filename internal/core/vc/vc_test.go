// Tests eddsa-jcs-2022 against the W3C published vector, and cross sign/verify with a one-byte alteration (DP-3, T-HUB-010).
// Fixtures: the vector files under conformance/vectors and a freshly generated key pair.
package vc

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
)

// vectorDir holds the W3C eddsa-jcs-2022 test vectors.
var vectorDir = filepath.Join("..", "..", "..", "conformance", "vectors", "eddsa-jcs-2022")

// keyPair is the W3C test key pair.
type keyPair struct {
	Public  string `json:"publicKeyMultibase"`
	Private string `json:"privateKeyMultibase"`
}

// loadVector reads the signed vector and its key pair.
func loadVector(t *testing.T) ([]byte, keyPair) {
	t.Helper()
	doc, err := os.ReadFile(filepath.Join(vectorDir, "signedJCS.json"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(vectorDir, "keyPair.json"))
	if err != nil {
		t.Fatal(err)
	}
	var kp keyPair
	if err := json.Unmarshal(raw, &kp); err != nil {
		t.Fatal(err)
	}
	return doc, kp
}

// sample is an unsecured listing-like credential.
func sample(t *testing.T) []byte {
	t.Helper()
	until := timex.MustInstant("2026-10-11T06:00:00Z")
	c := New(TypeVersionListing, "https://w3id.org/alexandria-vhub/v1", "did:key:z6Mk", timex.MustInstant("2026-10-10T06:00:00Z"), &until, map[string]any{"sequence": 418})
	b, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestW3CVectorVerifies(t *testing.T) {
	doc, kp := loadVector(t)
	pub, err := did.DecodeMultikey(kp.Public)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(doc, pub); err != nil {
		t.Fatalf("published vector does not verify: %v", err)
	}
}

func TestW3CPrivateKeyMatchesPublic(t *testing.T) {
	_, kp := loadVector(t)
	priv, err := did.DecodePrivateMultibase(kp.Private)
	if err != nil {
		t.Fatal(err)
	}
	if did.EncodeMultikey(priv.Public().(ed25519.PublicKey)) != kp.Public {
		t.Fatal("private key does not derive the published public key")
	}
}

func TestSignThenVerify(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signed, err := Sign(sample(t), Options("did:key:z6Mk#k", timex.MustInstant("2026-10-10T06:00:00Z")), priv)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(signed, pub); err != nil {
		t.Fatal(err)
	}
}

func TestAlteredByteFails(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)
	signed, _ := Sign(sample(t), Options("did:key:z6Mk#k", 0), priv)
	altered := bytes.Replace(signed, []byte("418"), []byte("419"), 1)
	if err := Verify(altered, pub); err == nil {
		t.Fatal("altered document verified")
	}
}

func TestWrongKeyFails(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	other, _, _ := ed25519.GenerateKey(nil)
	signed, _ := Sign(sample(t), Options("did:key:z6Mk#k", 0), priv)
	if err := Verify(signed, other); err == nil {
		t.Fatal("verified with another key")
	}
}

func TestSignIsDeterministic(t *testing.T) {
	_, priv, _ := ed25519.GenerateKey(nil)
	a, _ := Sign(sample(t), Options("did:key:z6Mk#k", 0), priv)
	b, _ := Sign(sample(t), Options("did:key:z6Mk#k", 0), priv)
	if !bytes.Equal(a, b) {
		t.Fatal("two signatures of the same document differ")
	}
}
