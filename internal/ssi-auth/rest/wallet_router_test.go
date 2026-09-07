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

func (m *mockWallet) ProcessOid4vci(_ context.Context, _ string) error {
	return m.err
}

func (m *mockWallet) ProcessOid4vp(_ context.Context, _ string) error {
	return m.err
}
func (m *mockWallet) RotateKey(context.Context, string, time.Duration) error { return m.err }
func (m *mockWallet) RevokeKey(context.Context, string) error                { return m.err }
func (m *mockWallet) PublishDid(context.Context, string) error               { return m.err }
func (m *mockWallet) UnpublishDid(context.Context, string) error             { return m.err }
func (m *mockWallet) GetDidState(context.Context, string) (wallet.DidState, error) {
	return wallet.DidState{}, m.err
}

func (m *mockWallet) AddServiceEndpoint(context.Context, string, wallet.ServiceEndpointPlan) error {
	return m.err
}
func (m *mockWallet) RemoveServiceEndpoint(context.Context, string, string) error { return m.err }
func (m *mockWallet) StoreCredential(context.Context, *wallet.CredentialImportPlan) error {
	return m.err
}

func (m *mockWallet) GetCredentialsByType(context.Context, string) ([]wallet.Credential, error) {
	return m.credentials, m.err
}

func (m *mockWallet) RequestDcpCredential(context.Context, *wallet.DcpCredentialRequestPlan) (string, error) {
	return "req-1", m.err
}

func (m *mockWallet) GetDcpRequestStatus(context.Context, string) (wallet.DcpRequestStatus, error) {
	return wallet.DcpRequestStatus{Status: "COMPLETED"}, m.err
}

func (m *mockWallet) CreateParticipant(context.Context, *wallet.ParticipantPlan) error {
	return m.err
}

func (m *mockWallet) GetParticipant(context.Context, string) (wallet.Participant, error) {
	return wallet.Participant{}, m.err
}
func (m *mockWallet) SetParticipantState(context.Context, string, bool) error { return m.err }
func (m *mockWallet) RegenerateParticipantToken(context.Context, string) (string, error) {
	return "new-tok", m.err
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

func TestNotImplementedInFafnirError(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{err: common.ErrNotImplementedInFafnir}
	engine := setupWalletRouter(mock)

	// Key rotation returns 501 with the exact message
	body := `{"duration":"1h"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/keys/k-1/rotate", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
	var errBody struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errBody.Error != "Error not implemented in fafnir wallet" {
		t.Fatalf("unexpected error message: %q", errBody.Error)
	}

	// DID publish returns 501 with the exact message
	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/did/did1/publish", nil)
	engine.ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec2.Code, http.StatusNotImplemented)
	}

	// Filtered credentials query returns 501 with the exact message
	rec3 := httptest.NewRecorder()
	req3 := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/vcs?type=CustomType", nil)
	engine.ServeHTTP(rec3, req3)
	if rec3.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec3.Code, http.StatusNotImplemented)
	}
}

func TestNotImplementedInIdentityHubError(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{err: common.ErrNotImplementedInIdentityHub}
	engine := setupWalletRouter(mock)

	body := `{"uri":"openid-credential-offer://test-vci"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/oid4vci", strings.NewReader(body))
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotImplemented {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotImplemented)
	}
	var errBody struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decoding error response: %v", err)
	}
	if errBody.Error != "Error not implemented in IdentityHub wallet" {
		t.Fatalf("unexpected error message: %q", errBody.Error)
	}
}

func TestIdentityHubEndpointsSuccess(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	// Key rotate
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/keys/k1/rotate", strings.NewReader(`{"duration":"2h"}`))
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Key revoke
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/keys/k1/revoke", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// DID publish & unpublish
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/did/did1/publish", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/did/did1/unpublish", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Store credential
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/credential", strings.NewReader(`{"id":"c1","format":"jwt","payload":{}}`))
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}

	// DCP request
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/dcp/request", strings.NewReader(`{"issuerUrl":"https://iss","holderPid":"pid1","types":["type1"]}`))
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}

	// Participants
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/participants", strings.NewReader(`{"id":"pid1","did":"did:web:ex","active":true}`))
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
}
