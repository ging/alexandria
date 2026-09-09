package identityhub

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"
	"time"
)

func TestNormalizeKey(t *testing.T) {
	// 1. RSA Key in PEM
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating rsa key: %v", err)
	}
	rsaPubDer, err := x509.MarshalPKIXPublicKey(&rsaPriv.PublicKey)
	if err != nil {
		t.Fatalf("marshaling rsa pub: %v", err)
	}
	rsaPubPem := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: rsaPubDer}))

	rsaDto := KeyPairDto{
		KeyID:               "rsa-key-1",
		PrivateKeyAlias:     "my-rsa-alias",
		SerializedPublicKey: rsaPubPem,
		Timestamp:           EpochMillis(time.Now().UnixMilli()),
	}
	rsaKey := normalizeKey(rsaDto)
	if rsaKey.Kty != "RSA" {
		t.Errorf("expected kty RSA, got %s", rsaKey.Kty)
	}
	if rsaKey.Crv != nil {
		t.Errorf("expected nil Crv for RSA, got %v", *rsaKey.Crv)
	}
	if rsaKey.Alias != "my-rsa-alias" {
		t.Errorf("expected alias 'my-rsa-alias', got %s", rsaKey.Alias)
	}

	// 2. Ed25519 in JWK JSON
	edJwk := `{"kty":"OKP","crv":"Ed25519","kid":"ed-key","x":"A3xPgplaUBuhjG2DZGSEVvVBUvyLkm4PAWKqk47aEEw"}`
	edDto := KeyPairDto{
		KeyID:               "ed-key-1",
		PrivateKeyAlias:     "ed-alias",
		SerializedPublicKey: edJwk,
	}
	edKey := normalizeKey(edDto)
	if edKey.Kty != "OKP" {
		t.Errorf("expected kty OKP, got %s", edKey.Kty)
	}
	if edKey.Crv == nil || *edKey.Crv != "Ed25519" {
		t.Errorf("expected crv Ed25519, got %v", edKey.Crv)
	}

	// 3. ECDSA P-256 in PEM
	ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating ec key: %v", err)
	}
	ecPubDer, err := x509.MarshalPKIXPublicKey(&ecPriv.PublicKey)
	if err != nil {
		t.Fatalf("marshaling ec pub: %v", err)
	}
	ecPubPem := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecPubDer}))

	ecDto := KeyPairDto{
		KeyID:               "ec-key-1",
		SerializedPublicKey: ecPubPem,
	}
	ecKey := normalizeKey(ecDto)
	if ecKey.Kty != "EC" {
		t.Errorf("expected kty EC, got %s", ecKey.Kty)
	}
	if ecKey.Crv == nil || *ecKey.Crv != "P-256" {
		t.Errorf("expected crv P-256, got %v", ecKey.Crv)
	}
	if ecKey.Alias != "ec-key-1" {
		t.Errorf("expected fallback alias 'ec-key-1', got %s", ecKey.Alias)
	}

	// 4. Fallback to Descriptor
	descDto := KeyPairDto{
		KeyID: "desc-key",
		Descriptor: &KeyDescriptorDto{
			Type: "EdDSA",
		},
	}
	descKey := normalizeKey(descDto)
	if descKey.Kty != "OKP" {
		t.Errorf("expected kty OKP from descriptor, got %s", descKey.Kty)
	}
	if descKey.Crv == nil || *descKey.Crv != "Ed25519" {
		t.Errorf("expected crv Ed25519, got %v", descKey.Crv)
	}
}
