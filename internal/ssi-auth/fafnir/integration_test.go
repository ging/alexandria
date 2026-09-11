//go:build integration

// integration_test exercises the Fafnir adapter against a live running Fafnir instance.
// It verifies real HTTP communication, default identity resolution, and unsupported stubs.
package fafnir_test

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
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/fafnir"
)

func fafnirBaseURL() string {
	port := os.Getenv("FAFNIR_HOST_PORT")
	if port == "" {
		port = "7003"
	}
	return fmt.Sprintf("http://127.0.0.1:%s", port)
}

func probeFafnir(t *testing.T, baseURL string) {
	t.Helper()

	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(baseURL + "/readiness")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skipf("skipping integration test: Fafnir not reachable at %s", baseURL)
	}
	_ = resp.Body.Close()
}

func TestFafnirLiveIntegration(t *testing.T) {
	baseURL := fafnirBaseURL()
	probeFafnir(t, baseURL)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	adapter, err := fafnir.New(baseURL, slog.Default())
	if err != nil {
		t.Fatalf("fafnir.New failed: %v", err)
	}
	defer func() { _ = adapter.Close() }()

	// 1. Verify live linking against Fafnir
	did, err := adapter.Link(ctx)
	if err != nil {
		t.Fatalf("Link failed against live Fafnir: %v", err)
	}
	if did.ID == "" {
		t.Fatal("expected non-empty DID from live Fafnir")
	}
	t.Logf("Linked against live Fafnir: %s", did.ID)

	// 2. Verify keys inventory
	keys, err := adapter.GetAllKeys(ctx)
	if err != nil {
		t.Fatalf("GetAllKeys failed: %v", err)
	}
	t.Logf("Fetched %d keys from live Fafnir", len(keys))

	// 3. Verify DIDs inventory
	dids, err := adapter.GetAllDids(ctx)
	if err != nil {
		t.Fatalf("GetAllDids failed: %v", err)
	}
	t.Logf("Fetched %d DIDs from live Fafnir", len(dids))

	// 4. Verify telemetry info
	info, err := adapter.WalletInfo(ctx)
	if err != nil {
		t.Fatalf("WalletInfo failed: %v", err)
	}
	t.Logf("Wallet info name: %s, permissions: %s", info.Name, info.Permission)

	// 5. Verify that IdentityHub-specific methods return ErrNotImplementedInFafnir
	if _, err := adapter.RotateKey(ctx, "key-1", time.Hour); !errors.Is(err, common.ErrNotImplementedInFafnir) {
		t.Errorf("expected ErrNotImplementedInFafnir for RotateKey, got %v", err)
	}
	if err := adapter.RevokeKey(ctx, "key-1"); !errors.Is(err, common.ErrNotImplementedInFafnir) {
		t.Errorf("expected ErrNotImplementedInFafnir for RevokeKey, got %v", err)
	}
	if _, err := adapter.PublishDid(ctx, "did:web:test"); !errors.Is(err, common.ErrNotImplementedInFafnir) {
		t.Errorf("expected ErrNotImplementedInFafnir for PublishDid, got %v", err)
	}
	if _, err := adapter.GetDidState(ctx, "did:web:test"); !errors.Is(err, common.ErrNotImplementedInFafnir) {
		t.Errorf("expected ErrNotImplementedInFafnir for GetDidState, got %v", err)
	}
	if _, err := adapter.CreateParticipant(ctx, nil); !errors.Is(err, common.ErrNotImplementedInFafnir) {
		t.Errorf("expected ErrNotImplementedInFafnir for CreateParticipant, got %v", err)
	}
}
