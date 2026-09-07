// wallet_router_test verifies HTTP routing, parameter parsing, and status codes.
// It exercises wallet endpoint handlers against a mock wallet service.
package rest_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/rest"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"github.com/gin-gonic/gin"
)

type mockWallet struct {
	credentials         []wallet.Credential
	gotDeleteCredential string
	err                 error
}

func (m *mockWallet) Link(context.Context) (wallet.Did, error)           { return wallet.Did{}, nil }
func (m *mockWallet) RegisterKey(context.Context, *wallet.KeyPlan) error { return nil }
func (m *mockWallet) GetAllKeys(context.Context) ([]wallet.Key, error)   { return nil, nil }
func (m *mockWallet) DeleteKey(context.Context, string) error            { return nil }
func (m *mockWallet) RegisterDid(context.Context, *wallet.DidPlan) error { return nil }
func (m *mockWallet) GetAllDids(context.Context) ([]wallet.Did, error)   { return nil, nil }
func (m *mockWallet) GetDidByID(context.Context, string) (wallet.Did, error) {
	return wallet.Did{}, nil
}
func (m *mockWallet) DeleteDid(context.Context, string) error                { return nil }
func (m *mockWallet) SetDefaultDid(context.Context, string) error            { return nil }
func (m *mockWallet) AddKeyToDid(context.Context, string, string) error      { return nil }
func (m *mockWallet) RemoveKeyFromDid(context.Context, string, string) error { return nil }
func (m *mockWallet) SetDefaultKey(context.Context, string, string) error    { return nil }
func (m *mockWallet) WalletInfo(context.Context) (wallet.WalletInfo, error) {
	return wallet.WalletInfo{}, nil
}
func (m *mockWallet) GetAllCredentials(context.Context) ([]wallet.Credential, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.credentials, nil
}
func (m *mockWallet) DeleteCredential(_ context.Context, id string) error {
	m.gotDeleteCredential = id
	return m.err
}
func (m *mockWallet) ProcessOid4vci(_ context.Context, uri string) error {
	return m.err
}
func (m *mockWallet) ProcessOid4vp(_ context.Context, uri string) error {
	return m.err
}

func setupWalletRouter(mock *mockWallet) *gin.Engine {
	gin.SetMode(gin.TestMode)
	svc := wallet.NewService(mock, nil, nil, nil)
	r := rest.NewWalletRouter(svc)

	engine := gin.New()
	group := engine.Group("")
	r.Register(group)
	return engine
}

func TestGetWalletCredentials(t *testing.T) {
	t.Parallel()

	now := time.Now().UTC().Truncate(time.Second)
	mock := &mockWallet{
		credentials: []wallet.Credential{
			{
				ID:             "vc-1",
				VcBody:         json.RawMessage(`"eyJhbGciOiJSU0EtT0FFUCJ9"`),
				VcType:         "gx:LegalPerson",
				VcFormat:       "jwt_vc_json",
				HolderDid:      "did:web:holder",
				IssuerDid:      "did:web:issuer",
				ParsedDocument: json.RawMessage(`{"type":["VerifiableCredential"]}`),
				ValidUntil:     &now,
				AddedOn:        now,
			},
		},
	}

	engine := setupWalletRouter(mock)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/vcs", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resps []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resps); err != nil {
		t.Fatalf("decoding body: %v", err)
	}

	if len(resps) != 1 {
		t.Fatalf("got %d credentials, want 1", len(resps))
	}

	if resps[0]["id"] != "vc-1" {
		t.Errorf("id = %v, want vc-1", resps[0]["id"])
	}
	if resps[0]["vcType"] != "gx:LegalPerson" {
		t.Errorf("vcType = %v, want gx:LegalPerson", resps[0]["vcType"])
	}
	if resps[0]["vcFormat"] != "jwt_vc_json" {
		t.Errorf("vcFormat = %v, want jwt_vc_json", resps[0]["vcFormat"])
	}
	if resps[0]["holderDid"] != "did:web:holder" {
		t.Errorf("holderDid = %v, want did:web:holder", resps[0]["holderDid"])
	}
	if resps[0]["issuerDid"] != "did:web:issuer" {
		t.Errorf("issuerDid = %v, want did:web:issuer", resps[0]["issuerDid"])
	}
}

func TestGetWalletCredentialsEmpty(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{credentials: []wallet.Credential{}}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/vcs", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if rec.Body.String() != "[]" {
		t.Errorf("body = %q, want []", rec.Body.String())
	}
}

func TestGetWalletCredentialsError(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{err: errors.New("backend failed")}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/vcs", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}

func TestDeleteCredential(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/wallet/credential/vc-to-delete", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	if mock.gotDeleteCredential != "vc-to-delete" {
		t.Errorf("deleted credential = %q, want vc-to-delete", mock.gotDeleteCredential)
	}
}

func TestDeleteCredentialError(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{err: common.ErrNotFound}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/wallet/credential/missing", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

func TestProcessOid4vci(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"uri":"openid-credential-offer://test-vci"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/oid4vci", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProcessOid4vciBadRequest(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"uri":""}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/oid4vci", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

func TestProcessOid4vp(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"uri":"openid4vp://test-vp"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/oid4vp", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestProcessOid4vpBadRequest(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"uri":"   "}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/oid4vp", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}
