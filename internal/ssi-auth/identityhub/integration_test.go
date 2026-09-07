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

	// 4. Verify DIDs inventory
	dids, err := adapter.GetAllDids(ctx)
	if err != nil {
		t.Fatalf("GetAllDids failed: %v", err)
	}
	t.Logf("Fetched %d DIDs for super-user from live IdentityHub", len(dids))

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

	if err := adapter.CreateParticipant(ctx, plan); err != nil {
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
	if err := adapter.SetParticipantState(ctx, testPid, false); err != nil {
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

	// 6. Verify that Fafnir-only methods return ErrNotImplementedInIdentityHub
	if err := adapter.ProcessOid4vci(ctx, "offer-uri"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for ProcessOid4vci, got %v", err)
	}
	if err := adapter.ProcessOid4vp(ctx, "presentation-uri"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for ProcessOid4vp, got %v", err)
	}
	if err := adapter.SetDefaultDid(ctx, "did:web:test"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for SetDefaultDid, got %v", err)
	}
	if err := adapter.SetDefaultKey(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for SetDefaultKey, got %v", err)
	}
	if err := adapter.AddKeyToDid(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for AddKeyToDid, got %v", err)
	}
	if err := adapter.RemoveKeyFromDid(ctx, "did:web:test", "key-1"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub for RemoveKeyFromDid, got %v", err)
	}
}
