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

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/config"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/identityhub"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
)

const defaultSuperUserKey = "c3VwZXItdXNlcg==.c3VwZXItdXNlci1zZWNyZXQta2V5LTEyMzQ1Njc4OTA="

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

func TestIdentityHubLiveIntegration(t *testing.T) {
	healthURL := identityHubHealthURL()
	probeIdentityHub(t, healthURL)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

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
	defer func() { _ = adapter.Close() }()

	// 1. Verify link / default identity
	did, err := adapter.Link(ctx)
	if err != nil {
		t.Fatalf("Link failed against live IdentityHub: %v", err)
	}
	if did.ID == "" {
		t.Fatal("expected non-empty DID from live IdentityHub")
	}
	t.Logf("Linked against live IdentityHub super-user DID: %s", did.ID)

	// 2. Verify wallet telemetry info
	info, err := adapter.WalletInfo(ctx)
	if err != nil {
		t.Fatalf("WalletInfo failed: %v", err)
	}
	t.Logf("Wallet info name: %s, ID: %s", info.Name, info.ID)

	// 3. Verify keys inventory
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

	// 4. Verify DIDs inventory
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

	// 5. Test participant provisioning and management
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

	// Query created participant
	part, err := adapter.GetParticipant(ctx, testPid)
	if err != nil {
		t.Fatalf("GetParticipant failed: %v", err)
	}
	if part.ID != testPid {
		t.Errorf("expected participant ID %s, got %s", testPid, part.ID)
	}

	// Deactivate participant
	if _, err := adapter.SetParticipantState(ctx, testPid, false); err != nil {
		t.Errorf("SetParticipantState deactivate failed: %v", err)
	}

	// Regenerate token for participant
	newToken, err := adapter.RegenerateParticipantToken(ctx, testPid)
	if err != nil {
		t.Errorf("RegenerateParticipantToken failed: %v", err)
	}
	if newToken == "" {
		t.Errorf("expected non-empty token from RegenerateParticipantToken")
	}

	// 6. Test RegisterKey with PEM key
	rsaKey := `-----BEGIN PRIVATE KEY-----
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

	keyID := fmt.Sprintf("test-key-%d", time.Now().UnixNano())
	keyPlan := &wallet.KeyPlan{
		ID:    keyID,
		Alias: "test-alias",
		Pem:   rsaKey,
	}
	if _, err := adapter.RegisterKey(ctx, keyPlan); err != nil {
		t.Fatalf("RegisterKey failed: %v", err)
	}
	t.Logf("Successfully registered key %s in IdentityHub", keyID)

	// 7. Verify RegisterDid with friendly alias (resolves to participant DID and publishes)
	if _, err := adapter.RegisterDid(ctx, &wallet.DidPlan{Alias: "hola"}); err != nil {
		t.Fatalf("RegisterDid with alias 'hola' failed: %v", err)
	}
	t.Logf("Successfully executed RegisterDid with alias 'hola'")

	// 7b. Verify RegisterDid with unmanaged DID returns ErrInvalidInput gracefully
	if _, err := adapter.RegisterDid(ctx, &wallet.DidPlan{Alias: "did:web:unmanaged.com"}); !errors.Is(err, common.ErrInvalidInput) {
		t.Errorf("expected ErrInvalidInput for unmanaged DID, got %v", err)
	}

	// 7c. Verify DID state
	didState, err := adapter.GetDidState(ctx, "did:web:super-user")
	if err != nil {
		t.Errorf("GetDidState failed: %v", err)
	} else {
		t.Logf("GetDidState for super-user: %s", didState.State)
	}

	// 7d. Verify AddServiceEndpoint
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

	// 8. Verify that Fafnir-only methods return ErrNotImplementedInIdentityHub
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
