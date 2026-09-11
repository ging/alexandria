// Package identityhub tests internal normalization logic for keys, credentials, and participants.
// It ensures compatibility with domain wallet entities and validates JSON constraints.
package identityhub

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"
)

func generateRSAPublicKeyPEM(t *testing.T) string {
	t.Helper()
	rsaPriv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating rsa key: %v", err)
	}
	rsaPubDer, err := x509.MarshalPKIXPublicKey(&rsaPriv.PublicKey)
	if err != nil {
		t.Fatalf("marshaling rsa pub: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: rsaPubDer}))
}

func generateECDSAPublicKeyPEM(t *testing.T) string {
	t.Helper()
	ecPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating ec key: %v", err)
	}
	ecPubDer, err := x509.MarshalPKIXPublicKey(&ecPriv.PublicKey)
	if err != nil {
		t.Fatalf("marshaling ec pub: %v", err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecPubDer}))
}

func TestNormalizeKey_RSA_PEM(t *testing.T) {
	rsaPubPem := generateRSAPublicKeyPEM(t)

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
}

func TestNormalizeKey_Ed25519_JWK(t *testing.T) {
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
}

func TestNormalizeKey_ECDSA_P256_PEM(t *testing.T) {
	ecPubPem := generateECDSAPublicKeyPEM(t)

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
}

func TestNormalizeKey_FallbackToDescriptor(t *testing.T) {
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

// TestNormalizeCredential_JwtRawVc verifies normalization of JWT credentials and ensures valid JSON fields.
func TestNormalizeCredential_JwtRawVc(t *testing.T) {
	t.Parallel()

	rawJwt := "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJkaWQ6d2ViOmlzc3VlciIsInN1YiI6ImRpZDp3ZWI6c3VwZXItdXNlciJ9.signature"
	dto := VerifiableCredentialResourceDto{
		ID:                   "cred-1",
		ParticipantContextID: "super-user",
		HolderID:             "did:web:super-user",
		IssuerID:             "did:web:issuer",
		VerifiableCredential: VerifiableCredentialBody{
			Format:     "VC1_0_JWT",
			RawVc:      rawJwt,
			Credential: json.RawMessage(`{"id":"cred-1","type":["VerifiableCredential","CustomType"],"issuanceDate":"2026-09-11T08:00:00Z"}`),
		},
	}

	cred := normalizeCredential(dto)
	if cred.ID != "cred-1" {
		t.Errorf("expected ID cred-1, got %s", cred.ID)
	}
	if !json.Valid(cred.VcBody) {
		t.Errorf("expected valid JSON for VcBody, got %s", string(cred.VcBody))
	}
	if !json.Valid(cred.Credential) {
		t.Errorf("expected valid JSON for Credential, got %s", string(cred.Credential))
	}
	if cred.VcType != "CustomType" {
		t.Errorf("expected vcType CustomType, got %s", cred.VcType)
	}
	if cred.IssuanceDate == nil {
		t.Error("expected non-nil IssuanceDate")
	}
}

// TestNormalizeCredential_JsonRawVc verifies normalization of plain JSON credentials.
func TestNormalizeCredential_JsonRawVc(t *testing.T) {
	t.Parallel()

	rawJSON := `{"id":"cred-2","type":["VerifiableCredential","TestType"],"issuanceDate":"2026-09-10T12:00:00Z"}`
	dto := VerifiableCredentialResourceDto{
		ID:                   "cred-2",
		ParticipantContextID: "super-user",
		HolderID:             "did:web:super-user",
		IssuerID:             "did:web:issuer",
		VerifiableCredential: VerifiableCredentialBody{
			Format: "JSON_LD",
			RawVc:  rawJSON,
		},
	}

	cred := normalizeCredential(dto)
	if cred.ID != "cred-2" {
		t.Errorf("expected ID cred-2, got %s", cred.ID)
	}
	if !json.Valid(cred.VcBody) {
		t.Errorf("expected valid JSON for VcBody, got %s", string(cred.VcBody))
	}
	if cred.VcType != "TestType" {
		t.Errorf("expected vcType TestType, got %s", cred.VcType)
	}
}
