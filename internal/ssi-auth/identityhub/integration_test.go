//go:build integration

// integration_test exercises the IdentityHub adapter against a live running Eclipse EDC IdentityHub instance.
// It verifies HTTP API communication, participant operations, key/DID retrieval, and unsupported stubs.
package identityhub_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/common"
	"github.com/ging/alexandria/internal/config"
	"github.com/ging/alexandria/internal/ssi-auth/identityhub"
	"github.com/ging/alexandria/internal/ssi-auth/wallet"
)

const defaultSuperUserKey = "c3VwZXItdXNlcg==.c3VwZXItdXNlci1zZWNyZXQta2V5LTEyMzQ1Njc4OTA="

const sampleRSAPrivateKeyPEM = `-----BEGIN PRIVATE KEY-----
MIIEvQIBADANBgkqhkiG9w0BAQEFAASCBKcwggSjAgEAAoIBAQDr//yYtu1E2ngC
mCZ0zpzvuE7D7XEj60ErhTz9bOYulpxQDNI6wy0tt5Xesgkp+RGs/KwozgOn6K9F
AXuYDyPcUL8/hZrFeMY18udjgCDryykywVNwgf9zDCwEu/59mBzCGh0DQLEC+qRv
7rgAet4CZKR2rNuyNuYmqXBkx8PfB7pfnWvpHqr9j34AopmtXpjc6zYp9yNSemlk
26P1dZrvGUkVfjNAzvUkkwkjTq1TyPIQGf/k5teO56LTDIjgA2CBpavGP/zQls81
1kDh9qrVFIlNx33bWRYG0d0txTsYzb2FLklL6mQTD2B76fQJKVB6TWMe6pKveoCt
CvJvZ1A5AgMBAAECggEAOHBSv07X9WRt2Oj8IWkb/PRN2etZ6GYlgrvtdwnpDnE0
VqyKRkVQ86L483YOXPxUrtMKdQO3uhsad11AaoAMam7hHdbcyab1eAdsMM5+kQVY
B+xWAQ0Fw0TA7izrUqvjDMRj9dgtvPGmC6LCXFMF7vqUnlD+hWM9rTdOSru/awGf
dqRUQfKyhPMqgZ8XNgpDBQ+656wGDpWbaMwD41+7oNkgVNch7l86X4hjmHnZV1Ur
wiDq3NOvHgj+H9JAQSMmyvtSCVlddrovsuz4w4TqemZ2T+KnSAZuMJFW+WP/6RG7
0KUS43PZfYl5mQsS/KNzfqlsmWn5G/GKpV0eONYZVQKBgQD/45etPkGJCN8KmiZa
Svt9AV4FODw/NFzHtL4nIcl/Z3Hxdn9XuYAYb1FNHDwbt/s+EL4JOTtL0kLTovkz
SDp/eYuGaz2j1WeZ8VFxjTlkrJ6FeJqHyzphIKq3iyusuwf8bofqRJWRrXeM7jkF
sGb4DPudrYNSUrm51N8pJh6OrwKBgQDsGi+s5Ecl2VoBNswS312nAAudku4BCO+v
RWY+XCXHNIjs/pLQIrjGWdca7elDdvIw/s8mWmqvZfEq1wRz96yGT7leIyQUecLL
JR0+AD/tp/8X1gJcHzeUXLH1leBPL5pCWSFfXPDxBDYdgSsftlQXBkl4K5l8V2rm
c7BwpLMJlwKBgQDx0jWO5Ryt0hJmRJMmFWJhGh+uMxzMZkGgATEKbiWsHyhRFrj1
QDrL3Lcqdhpf35ixaMUOlmVxG/1HX+a9De8qdMTkfQg9gflsQ9/BvcKVX4RXgkgX
OHmtPF/ZIM5faEj9x77uJ25pw1MNfjupIrHMjQhkVIucCs21znQuwPVzxQKBgByW
IxWc4hxsD6C8AMN8NfulXsKqapTHfzXKglGkmJJhAv8m56G5woOJlyjUi3y2pyZV
g8FSCz7HagbU194uq73rYzdJq/GquHIeQUcjgpoE0DcTm1+KDBGzk3x3tBwCWHwW
DJteRnH4H5E89Xq2ecH76eNZ7BCJCRF0CnXpCyBrAoGAS5fTkGwoozeW/4EsACeZ
K1euT3oE7g4VPt3bipUAC1yI4T9h3piytrTWeHzBwXYHpQIEOx/rmN1mmmqXI7bs
/BKI+UXMTgFj5m+NtZ8+LofID06B+i2D7IG1Dj9nDVuVwruy7cY6S1N6aTLQwZnt
ukjWvXiJy97LtDuP8YYWBnI=
-----END PRIVATE KEY-----`

func identityHubIdentityURL() string {
	val := os.Getenv("IDENTITYHUB_IDENTITY_URL")
	if val != "" {
		return val
	}
	return "http://127.0.0.1:8182/api/identity/v1"
}

func identityHubHealthURL() string {
	val := os.Getenv("IDENTITYHUB_HEALTH_URL")
	if val != "" {
		return val
	}
	return "http://127.0.0.1:8181/api/check/health"
}

func identityHubAPIKey() string {
	val := os.Getenv("IDENTITYHUB_API_KEY")
	if val != "" {
		return val
	}
	return defaultSuperUserKey
}

func probeIdentityHub(t *testing.T, healthURL string) {
	t.Helper()

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(healthURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("skipping integration test: IdentityHub not reachable at %s", healthURL)
	}
	_ = resp.Body.Close()
}

func setupLiveIntegration(t *testing.T) (wallet.Wallet, context.Context) {
	t.Helper()

	healthURL := identityHubHealthURL()
	probeIdentityHub(t, healthURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	identityURL := identityHubIdentityURL()
	apiKey := identityHubAPIKey()

	cfg := &config.IdentityHubConfig{
		IdentityAPIURL: identityURL,
		APIKey:         apiKey,
		ParticipantID:  "super-user",
	}

	adapter, err := identityhub.New(cfg, slog.Default())
	if err != nil {
		t.Fatalf("identityhub.New failed: %v", err)
	}
	t.Cleanup(func() { _ = adapter.Close() })

	return adapter, ctx
}

func TestIdentityHubLive_LinkAndWalletInfo(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	did, err := adapter.Link(ctx)
	if err != nil {
		t.Fatalf("Link failed against live IdentityHub: %v", err)
	}
	if did.ID == "" {
		t.Fatal("expected non-empty DID from live IdentityHub")
	}
	t.Logf("Linked against live IdentityHub super-user DID: %s", did.ID)

	info, err := adapter.WalletInfo(ctx)
	if err != nil {
		t.Fatalf("WalletInfo failed: %v", err)
	}
	t.Logf("Wallet info name: %s, ID: %s", info.Name, info.ID)
}

func TestIdentityHubLive_KeysInventory(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	keys, err := adapter.GetAllKeys(ctx)
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}
	t.Logf("Fetched %d keys for super-user from live IdentityHub", len(keys))
	for _, k := range keys {
		crvStr := "none"
		if k.Crv != nil {
			crvStr = *k.Crv
		}
		t.Logf("Key %s: kty=%s, crv=%s, alias=%s", k.ID, k.Kty, crvStr, k.Alias)
	}
}

func TestIdentityHubLive_DidsInventory(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	dids, err := adapter.GetAllDids(ctx)
	if err != nil {
		t.Fatalf("GetAllDids failed: %v", err)
	}
	t.Logf("Fetched %d DIDs for super-user from live IdentityHub", len(dids))
	if len(dids) > 0 {
		t.Logf("DID %s has %d verificationMethods and %d keys", dids[0].ID, len(dids[0].Document.VerificationMethod), len(dids[0].Keys))
		if len(dids[0].Document.VerificationMethod) == 0 {
			t.Errorf("expected DID document to contain verificationMethods, got 0")
		}
	}
}

func TestIdentityHubLive_ParticipantManagement(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	testPid := fmt.Sprintf("test-part-%d", time.Now().UnixNano())
	testDid := fmt.Sprintf("did:web:%s", testPid)
	plan := &wallet.ParticipantPlan{
		ID:     testPid,
		Did:    testDid,
		Active: true,
		KeyDescriptor: &wallet.KeyDescriptor{
			KeyID: fmt.Sprintf("%s-key", testPid),
			Type:  "EdDSA",
			Properties: map[string]any{
				"algorithm": "EdDSA",
				"curve":     "Ed25519",
			},
		},
	}

	if _, err := adapter.CreateParticipant(ctx, plan); err != nil {
		t.Fatalf("CreateParticipant failed: %v", err)
	}
	t.Logf("Successfully provisioned participant %s", testPid)

	part, err := adapter.GetParticipant(ctx, testPid)
	if err != nil {
		t.Fatalf("GetParticipant failed: %v", err)
	}
	if part.ID != testPid {
		t.Errorf("expected participant ID %s, got %s", testPid, part.ID)
	}

	if _, err := adapter.SetParticipantState(ctx, testPid, false); err != nil {
		t.Errorf("SetParticipantState deactivate failed: %v", err)
	}

	newToken, err := adapter.RegenerateParticipantToken(ctx, testPid)
	if err != nil {
		t.Errorf("RegenerateParticipantToken failed: %v", err)
	}
	if newToken == "" {
		t.Errorf("expected non-empty token from RegenerateParticipantToken")
	}
}

func TestIdentityHubLive_RegisterKey(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	keyID := fmt.Sprintf("test-key-%d", time.Now().UnixNano())
	keyPlan := &wallet.KeyPlan{
		ID:    keyID,
		Alias: "test-alias",
		Pem:   sampleRSAPrivateKeyPEM,
	}
	if _, err := adapter.RegisterKey(ctx, keyPlan); err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}
	t.Logf("Successfully registered key %s in IdentityHub", keyID)
}

func TestIdentityHubLive_RegisterJwkKey(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	keyID := fmt.Sprintf("test-jwk-%d", time.Now().UnixNano())
	jwkStr := fmt.Sprintf(`{"kty":"OKP","crv":"Ed25519","kid":"%s","x":"9nhnpQlR-MdqxM6qmqftTdCmCphBkJTzwP6855UXoSk"}`, keyID)
	keyPlan := &wallet.KeyPlan{
		ID:    keyID,
		Alias: "test-jwk-alias",
		Pem:   jwkStr,
	}
	registered, err := adapter.RegisterKey(ctx, keyPlan)
	if err != nil {
		t.Fatalf("RegisterKey with JWK failed: %v", err)
	}
	if registered.Kty != "OKP" {
		t.Errorf("expected Kty OKP, got %s", registered.Kty)
	}
	t.Logf("Successfully registered JWK key %s in IdentityHub", keyID)
}

func TestIdentityHubLive_RotateAndRevokeKey(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	keyID := fmt.Sprintf("test-rot-%d", time.Now().UnixNano())
	keyPlan := &wallet.KeyPlan{
		ID:    keyID,
		Alias: "test-rot-alias",
		Pem:   sampleRSAPrivateKeyPEM,
	}
	if _, err := adapter.RegisterKey(ctx, keyPlan); err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}

	rotated, err := adapter.RotateKey(ctx, keyID, 24*time.Hour)
	if err != nil {
		t.Fatalf("RotateKey failed: %v", err)
	}
	t.Logf("Successfully rotated key %s, state: %s", keyID, rotated.State)

	if err := adapter.RevokeKey(ctx, keyID); err != nil {
		t.Fatalf("RevokeKey failed: %v", err)
	}
	t.Logf("Successfully revoked key %s", keyID)
}

func TestIdentityHubLive_RegisterDidAndEndpoints(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	// Verify RegisterDid with friendly alias (resolves to participant DID and publishes)
	if _, err := adapter.RegisterDid(ctx, &wallet.DidPlan{Alias: "hola"}); err != nil {
		t.Fatalf("RegisterDid with alias 'hola' failed: %v", err)
	}
	t.Logf("Successfully executed RegisterDid with alias 'hola'")

	// Verify RegisterDid with unmanaged DID returns ErrInvalidInput gracefully
	if _, err := adapter.RegisterDid(ctx, &wallet.DidPlan{Alias: "did:web:unmanaged.com"}); !errors.Is(err, common.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for unmanaged DID, got %v", err)
	}

	// Verify DID state
	didState, err := adapter.GetDidState(ctx, "did:web:super-user")
	if err != nil {
		t.Errorf("GetDidState failed: %v", err)
	} else {
		t.Logf("GetDidState for super-user: %s", didState.State)
	}

	// Verify AddServiceEndpoint
	epID := fmt.Sprintf("ep-live-test-%d", time.Now().UnixNano())
	epPlan := wallet.ServiceEndpointPlan{
		ID:   epID,
		Type: "CredentialService",
		URL:  "https://alexandria.example.com/credentials",
	}
	if _, err := adapter.AddServiceEndpoint(ctx, "did:web:super-user", epPlan); err != nil {
		t.Errorf("AddServiceEndpoint failed: %v", err)
	} else {
		t.Logf("Successfully added service endpoint to did:web:super-user")
		// Clean up endpoint
		if _, err := adapter.RemoveServiceEndpoint(ctx, "did:web:super-user", epID); err != nil {
			t.Logf("Warning: RemoveServiceEndpoint clean-up failed: %v", err)
		}
	}
}

func TestIdentityHubLive_UnsupportedOperations(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	if err := adapter.ProcessOid4vci(ctx, "offer-uri"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for ProcessOid4vci, got %v", err)
	}
	if err := adapter.ProcessOid4vp(ctx, "presentation-uri"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for ProcessOid4vp, got %v", err)
	}
	if _, err := adapter.SetDefaultDid(ctx, "did:web:test"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for SetDefaultDid, got %v", err)
	}
	if _, err := adapter.SetDefaultKey(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for SetDefaultKey, got %v", err)
	}
	if _, err := adapter.AddKeyToDid(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for AddKeyToDid, got %v", err)
	}
	if _, err := adapter.RemoveKeyFromDid(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for RemoveKeyFromDid, got %v", err)
	}
}

func TestIdentityHubLive_CredentialLifecycle(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	credID := fmt.Sprintf("cred-live-%d", time.Now().UnixNano())
	plan := &wallet.CredentialImportPlan{
		ID:     credID,
		Format: "VC1_0_JWT",
		RawVc:  "eyJhbGciOiJSUzI1NiJ9.e30.c2ln",
	}

	cred, err := adapter.StoreCredential(ctx, plan)
	if err != nil {
		t.Fatalf("StoreCredential failed: %v", err)
	}
	t.Logf("Successfully stored credential %s in IdentityHub", cred.ID)

	creds, err := adapter.GetAllCredentials(ctx)
	if err != nil {
		t.Fatalf("GetAllCredentials failed: %v", err)
	}
	t.Logf("Live IdentityHub credentials count: %d", len(creds))

	if err := adapter.DeleteCredential(ctx, credID); err != nil {
		t.Errorf("DeleteCredential failed: %v", err)
	}
}

func TestIdentityHubLive_DcpRequest(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	testPid := fmt.Sprintf("dcp-part-%d", time.Now().UnixNano())
	if _, err := adapter.CreateParticipant(ctx, &wallet.ParticipantPlan{
		ID:     testPid,
		Did:    fmt.Sprintf("did:web:%s", testPid),
		Active: true,
	}); err != nil {
		t.Fatalf("CreateParticipant for DCP test failed: %v", err)
	}
	defer func() {
		if ih, ok := adapter.(*identityhub.Adapter); ok {
			_ = ih.Client().DeleteParticipant(ctx, testPid)
		}
	}()

	plan := &wallet.DcpCredentialRequestPlan{
		HolderPid: testPid,
		IssuerDid: "did:web:issuer",
		Types:     []string{"MembershipCredential"},
		Format:    "VC1_0_JWT",
	}

	reqID, err := adapter.RequestDcpCredential(ctx, plan)
	if err != nil {
		t.Fatalf("RequestDcpCredential failed: %v", err)
	}
	t.Logf("Successfully initiated DCP request: %s", reqID)

	status, err := adapter.GetDcpRequestStatus(ctx, reqID)
	if err != nil {
		t.Fatalf("GetDcpRequestStatus failed: %v", err)
	}
	t.Logf("DCP request status for %s: %s", status.RequestID, status.Status)
}

func TestIdentityHubLive_UnpublishAndPublishDid(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	unpubState, err := adapter.UnpublishDid(ctx, "did:web:super-user")
	if err != nil {
		t.Fatalf("UnpublishDid failed: %v", err)
	}
	t.Logf("UnpublishDid state: %s", unpubState.State)

	pubState, err := adapter.PublishDid(ctx, "did:web:super-user")
	if err != nil {
		t.Fatalf("PublishDid restore failed: %v", err)
	}
	t.Logf("PublishDid restore state: %s", pubState.State)
}

func TestIdentityHubLive_CreateParticipantDefaultKey(t *testing.T) {
	adapter, ctx := setupLiveIntegration(t)

	testPid := fmt.Sprintf("test-auto-%d", time.Now().UnixNano())
	plan := &wallet.ParticipantPlan{
		ID:     testPid,
		Did:    fmt.Sprintf("did:web:%s", testPid),
		Active: true,
	}

	part, err := adapter.CreateParticipant(ctx, plan)
	if err != nil {
		t.Fatalf("CreateParticipant with default key failed: %v", err)
	}
	t.Logf("Successfully provisioned participant %s with default key", part.ID)
	if ih, ok := adapter.(*identityhub.Adapter); ok {
		_ = ih.Client().DeleteParticipant(ctx, testPid)
	}
}
