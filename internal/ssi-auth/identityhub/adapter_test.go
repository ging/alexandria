// adapter_test exercises IdentityHub adapter methods against a mock HTTP server.
// It verifies serialization, endpoint dispatching, and error mappings.
package identityhub_test

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/common"
	"github.com/ging/alexandria/internal/config"
	"github.com/ging/alexandria/internal/ssi-auth/identityhub"
	"github.com/ging/alexandria/internal/ssi-auth/wallet"
	"github.com/trustbloc/did-go/doc/did"
)

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func newMockIdentityHubMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/participants/test-pid", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		writeJSON(w, identityhub.ParticipantContextDto{
			ParticipantContextID: "test-pid",
			Did:                  "did:web:example.com",
			State:                "ACTIVATED",
			Roles:                []string{"admin"},
			CreatedAt:            identityhub.EpochMillis(time.Now().UnixMilli()),
		})
	})

	mux.HandleFunc("/participants/p-2", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, identityhub.ParticipantContextDto{
			ParticipantContextID: "p-2",
			Did:                  "did:web:p2",
			State:                "ACTIVATED",
			Roles:                []string{"admin"},
			CreatedAt:            identityhub.EpochMillis(time.Now().UnixMilli()),
		})
	})

	mux.HandleFunc("/participants/test-pid/keypairs", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, []identityhub.KeyPairDto{
				{
					ID:    "key-1",
					KeyID: "key-1",
					State: "ACTIVE",
					Descriptor: &identityhub.KeyDescriptorDto{
						Type: "OKP",
					},
				},
			})
			return
		}
		if r.Method == http.MethodPut {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/participants/test-pid/keypairs/key-1/rotate", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/keypairs/key-1/revoke", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/dids/publish", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/dids/unpublish", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/dids/state", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, identityhub.DidStateDto{
			Did:   "did:web:example.com",
			State: "PUBLISHED",
		})
	})

	mux.HandleFunc("/participants/test-pid/dids/query", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, []any{
			map[string]any{
				"id":       "did:web:example.com",
				"@context": []string{"https://www.w3.org/ns/did/v1"},
				"verificationMethod": []any{
					map[string]any{
						"id":         "key-1",
						"type":       "JsonWebKey2020",
						"controller": "did:web:example.com",
						"publicKeyJwk": map[string]any{
							"kty": "RSA",
							"e":   "AQAB",
							"n":   "AQAB",
						},
					},
				},
				"authentication": []string{"key-1"},
				"service":        []any{},
			},
		})
	})

	encodedTestDid := base64.RawURLEncoding.EncodeToString([]byte("did:web:example.com"))
	mux.HandleFunc(fmt.Sprintf("/participants/test-pid/dids/%s/endpoints", encodedTestDid), func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/credentials", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeJSON(w, []identityhub.VerifiableCredentialResourceDto{
				{
					ID:       "cred-1",
					HolderID: "did:web:holder",
					IssuerID: "did:web:issuer",
					VerifiableCredential: identityhub.VerifiableCredentialBody{
						Format: "VC1_0_JWT",
						RawVc:  `{"id":"cred-1"}`,
					},
				},
			})
			return
		}
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/participants/test-pid/credentials/cred-1", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/participants", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/participants/test-pid/state", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("/participants/test-pid/token", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, identityhub.TokenResponseDto{Token: "new-token-123"})
	})

	mux.HandleFunc(fmt.Sprintf("/participants/test-pid/dids/%s/endpoints/ep-1", encodedTestDid), func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusMethodNotAllowed)
	})

	mux.HandleFunc("/participants/test-pid/credentials/request", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{
			"requestId": "req-123",
		})
	})

	mux.HandleFunc("/participants/test-pid/credentials/request/req-123", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, identityhub.DcpRequestStatusDto{
			RequestID: "req-123",
			Status:    "COMPLETED",
		})
	})

	return mux
}

func setupTestAdapter(t *testing.T) (wallet.Wallet, context.Context) {
	t.Helper()
	srv := httptest.NewServer(newMockIdentityHubMux())
	t.Cleanup(srv.Close)

	adapter, err := identityhub.New(&config.IdentityHubConfig{
		IdentityAPIURL: srv.URL,
		APIKey:         "secret",
		ParticipantID:  "test-pid",
	}, nil)
	if err != nil {
		t.Fatalf("unexpected init error: %v", err)
	}
	t.Cleanup(func() { _ = adapter.Close() })

	return adapter, context.Background()
}

func TestAdapter_LinkAndWalletInfo(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	did, err := adapter.Link(ctx)
	if err != nil {
		t.Fatalf("Link failed: %v", err)
	}
	if did.ID != "did:web:example.com" {
		t.Errorf("expected did:web:example.com, got %s", did.ID)
	}

	info, err := adapter.WalletInfo(ctx)
	if err != nil {
		t.Fatalf("WalletInfo failed: %v", err)
	}
	if info.ID != "test-pid" {
		t.Errorf("expected participant test-pid, got %s", info.ID)
	}
}

func TestAdapter_Keys(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	keys, err := adapter.GetAllKeys(ctx)
	if err != nil || len(keys) != 1 {
		t.Fatalf("GetAllKeys failed: %v, len=%d", err, len(keys))
	}
	if _, err := adapter.RegisterKey(ctx, &wallet.KeyPlan{ID: "key-2", Pem: "mock"}); err != nil {
		t.Errorf("RegisterKey failed: %v", err)
	}
	if _, err := adapter.RotateKey(ctx, "key-1", 10*time.Minute); err != nil {
		t.Errorf("RotateKey failed: %v", err)
	}
	if err := adapter.RevokeKey(ctx, "key-1"); err != nil {
		t.Errorf("RevokeKey failed: %v", err)
	}
}

func TestAdapter_Dids(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	if _, err := adapter.PublishDid(ctx, "did:web:example.com"); err != nil {
		t.Errorf("PublishDid failed: %v", err)
	}
	st, err := adapter.GetDidState(ctx, "did:web:example.com")
	if err != nil || st.State != wallet.DidStatePublished {
		t.Errorf("GetDidState failed: %v, state=%s", err, st.State)
	}
	if _, err := adapter.AddServiceEndpoint(ctx, "did:web:example.com", wallet.ServiceEndpointPlan{
		ID:   "ep-1",
		Type: "CredentialService",
		URL:  "https://example.com/ep",
	}); err != nil {
		t.Errorf("AddServiceEndpoint failed: %v", err)
	}
	if _, err := adapter.RemoveServiceEndpoint(ctx, "did:web:example.com", "ep-1"); err != nil {
		t.Errorf("RemoveServiceEndpoint failed: %v", err)
	}
	if _, err := adapter.UnpublishDid(ctx, "did:web:example.com"); err != nil {
		t.Errorf("UnpublishDid failed: %v", err)
	}
}

func TestAdapter_Credentials(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	creds, err := adapter.GetAllCredentials(ctx)
	if err != nil || len(creds) != 1 {
		t.Fatalf("GetAllCredentials failed: %v", err)
	}
	byType, err := adapter.GetCredentialsByType(ctx, "MembershipCredential")
	if err != nil || len(byType) != 1 {
		t.Errorf("GetCredentialsByType failed: %v", err)
	}
	if _, err := adapter.StoreCredential(ctx, &wallet.CredentialImportPlan{ID: "cred-2", Payload: []byte(`{}`)}); err != nil {
		t.Errorf("StoreCredential failed: %v", err)
	}
	if err := adapter.DeleteCredential(ctx, "cred-1"); err != nil {
		t.Errorf("DeleteCredential failed: %v", err)
	}
}

func TestAdapter_Participants(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	if _, err := adapter.CreateParticipant(ctx, &wallet.ParticipantPlan{ID: "p-2", Did: "did:web:p2"}); err != nil {
		t.Errorf("CreateParticipant failed: %v", err)
	}
	part, err := adapter.GetParticipant(ctx, "test-pid")
	if err != nil || part.ID != "test-pid" {
		t.Errorf("GetParticipant failed: %v", err)
	}
	if _, err := adapter.SetParticipantState(ctx, "test-pid", false); err != nil {
		t.Errorf("SetParticipantState failed: %v", err)
	}
	tok, err := adapter.RegenerateParticipantToken(ctx, "test-pid")
	if err != nil || tok != "new-token-123" {
		t.Errorf("RegenerateParticipantToken failed: %v, tok=%s", err, tok)
	}

	if err := adapter.UpdateParticipantToken(ctx, "test-pid", "manual-token"); err != nil {
		t.Errorf("UpdateParticipantToken failed: %v", err)
	}
}

func TestAdapter_DCP(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	reqID, err := adapter.RequestDcpCredential(ctx, &wallet.DcpCredentialRequestPlan{
		IssuerURL: "https://issuer",
		Types:     []string{"MembershipCredential"},
	})
	if err != nil || reqID != "req-123" {
		t.Fatalf("RequestDcpCredential failed: %v, id=%s", err, reqID)
	}
	dcpSt, err := adapter.GetDcpRequestStatus(ctx, reqID)
	if err != nil || dcpSt.Status != "COMPLETED" {
		t.Errorf("GetDcpRequestStatus failed: %v", err)
	}
}

func TestAdapter_UnsupportedOperations(t *testing.T) {
	adapter, ctx := setupTestAdapter(t)

	if err := adapter.ProcessOid4vci(ctx, "oid4vci://offer"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub, got %v", err)
	}
	if err := adapter.ProcessOid4vp(ctx, "oid4vp://request"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub, got %v", err)
	}
	if _, err := adapter.SetDefaultDid(ctx, "did:web:default"); !errors.Is(err, common.ErrNotImplementedInIdentityHub) {
		t.Errorf("expected ErrNotImplementedInIdentityHub, got %v", err)
	}
}

func TestParseIdentityHubDoc(t *testing.T) {
	raw := []byte(`{"id":"did:web:super-user","service":[],"verificationMethod":[{"id":"super-user-key","type":"JsonWebKey2020","controller":"did:web:super-user","publicKeyMultibase":null,"publicKeyJwk":{"kty":"OKP","crv":"Ed25519","kid":"super-user-key","x":"A3xPgplaUBuhjG2DZGSEVvVBUvyLkm4PAWKqk47aEEw"}}],"authentication":["super-user-key"],"capabilityInvocation":["super-user-key"],"@context":["https://www.w3.org/ns/did/v1"]}`)
	doc, err := did.ParseDocument(raw)
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}
	t.Logf("Parsed doc ID: %s, verificationMethods: %d", doc.ID, len(doc.VerificationMethod))
}
