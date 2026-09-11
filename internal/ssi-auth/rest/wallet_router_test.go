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
	keys                []wallet.Key
	info                wallet.WalletInfo
	gotDeleteCredential string
	lastImportPlan      *wallet.CredentialImportPlan
	lastDcpPlan         *wallet.DcpCredentialRequestPlan
	lastParticipantPlan *wallet.ParticipantPlan
	lastDidID           string
	lastEndpointID      string
	err                 error
}

func (m *mockWallet) Link(context.Context) (wallet.Did, error) { return wallet.Did{}, nil }
func (m *mockWallet) RegisterKey(context.Context, *wallet.KeyPlan) (wallet.Key, error) {
	return wallet.Key{}, m.err
}

func (m *mockWallet) GetAllKeys(context.Context) ([]wallet.Key, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.keys, nil
}
func (m *mockWallet) DeleteKey(context.Context, string) error { return nil }
func (m *mockWallet) RegisterDid(context.Context, *wallet.DidPlan) (wallet.Did, error) {
	return wallet.Did{}, m.err
}
func (m *mockWallet) GetAllDids(context.Context) ([]wallet.Did, error) { return nil, nil }
func (m *mockWallet) GetDidByID(context.Context, string) (wallet.Did, error) {
	return wallet.Did{}, nil
}
func (m *mockWallet) DeleteDid(context.Context, string) error { return nil }
func (m *mockWallet) SetDefaultDid(context.Context, string) (wallet.Did, error) {
	return wallet.Did{}, m.err
}

func (m *mockWallet) AddKeyToDid(context.Context, string, string) (wallet.Did, error) {
	return wallet.Did{}, m.err
}

func (m *mockWallet) RemoveKeyFromDid(context.Context, string, string) (wallet.Did, error) {
	return wallet.Did{}, m.err
}

func (m *mockWallet) SetDefaultKey(context.Context, string, string) (wallet.Did, error) {
	return wallet.Did{}, m.err
}

func (m *mockWallet) WalletInfo(context.Context) (wallet.WalletInfo, error) {
	if m.err != nil {
		return wallet.WalletInfo{}, m.err
	}
	return m.info, nil
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

func (m *mockWallet) RotateKey(context.Context, string, time.Duration) (wallet.Key, error) {
	return wallet.Key{}, m.err
}
func (m *mockWallet) RevokeKey(context.Context, string) error { return m.err }
func (m *mockWallet) PublishDid(context.Context, string) (wallet.DidState, error) {
	return wallet.DidState{State: "PUBLISHED"}, m.err
}

func (m *mockWallet) UnpublishDid(context.Context, string) (wallet.DidState, error) {
	return wallet.DidState{State: "UNPUBLISHED"}, m.err
}

func (m *mockWallet) GetDidState(context.Context, string) (wallet.DidState, error) {
	return wallet.DidState{}, m.err
}

func (m *mockWallet) AddServiceEndpoint(context.Context, string, wallet.ServiceEndpointPlan) (wallet.Did, error) {
	return wallet.Did{}, m.err
}

func (m *mockWallet) RemoveServiceEndpoint(_ context.Context, didID, endpointID string) (wallet.Did, error) {
	m.lastDidID = didID
	m.lastEndpointID = endpointID
	return wallet.Did{}, m.err
}

func (m *mockWallet) StoreCredential(_ context.Context, plan *wallet.CredentialImportPlan) (wallet.Credential, error) {
	m.lastImportPlan = plan
	return wallet.Credential{ID: plan.ID}, m.err
}

func (m *mockWallet) GetCredentialsByType(context.Context, string) ([]wallet.Credential, error) {
	return m.credentials, m.err
}

func (m *mockWallet) RequestDcpCredential(_ context.Context, plan *wallet.DcpCredentialRequestPlan) (string, error) {
	m.lastDcpPlan = plan
	return "req-1", m.err
}

func (m *mockWallet) GetDcpRequestStatus(context.Context, string) (wallet.DcpRequestStatus, error) {
	return wallet.DcpRequestStatus{Status: "COMPLETED"}, m.err
}

func (m *mockWallet) CreateParticipant(_ context.Context, plan *wallet.ParticipantPlan) (wallet.Participant, error) {
	m.lastParticipantPlan = plan
	return wallet.Participant{ID: plan.ID, Did: plan.Did, Active: plan.Active}, m.err
}

func (m *mockWallet) GetParticipant(context.Context, string) (wallet.Participant, error) {
	return wallet.Participant{}, m.err
}

func (m *mockWallet) SetParticipantState(context.Context, string, bool) (wallet.Participant, error) {
	return wallet.Participant{}, m.err
}

func (m *mockWallet) RegenerateParticipantToken(context.Context, string) (string, error) {
	return "new-tok", m.err
}

func (m *mockWallet) UpdateParticipantToken(context.Context, string, string) error {
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

// TestGetWalletCredentials_RawJwtVcBody verifies that raw JWT strings in VcBody serialize safely to JSON.
func TestGetWalletCredentials_RawJwtVcBody(t *testing.T) {
	t.Parallel()

	rawJwt := "eyJhbGciOiJSUzI1NiJ9.eyJpc3MiOiJkaWQ6d2ViOmlzc3VlciIsInN1YiI6ImRpZDp3ZWI6c3VwZXItdXNlciJ9.signature"
	mock := &mockWallet{
		credentials: []wallet.Credential{
			{
				ID:        "vc-jwt",
				RawVc:     rawJwt,
				VcBody:    json.RawMessage(rawJwt),
				HolderDid: "did:web:holder",
				IssuerDid: "did:web:issuer",
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
		t.Fatalf("decoding body: %v (body was %q)", err, rec.Body.String())
	}
	if len(resps) != 1 {
		t.Fatalf("got %d credentials, want 1", len(resps))
	}
	if resps[0]["id"] != "vc-jwt" {
		t.Errorf("id = %v, want vc-jwt", resps[0]["id"])
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
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
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
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/did/did1/unpublish", nil)
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
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

	// Update participant token
	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(t.Context(), http.MethodPut, "/wallet/participants/pid1/token", strings.NewReader(`{"token":"manual-tok"}`))
	engine.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestGetWalletInfoExcludesPermissionAndAddedAt ensures removed metadata fields are absent from JSON.
func TestGetWalletInfoExcludesPermissionAndAddedAt(t *testing.T) {
	t.Parallel()

	now := time.Now()
	mock := &mockWallet{
		info: wallet.WalletInfo{
			ID:         "wid1",
			Name:       "wname",
			CreatedAt:  now,
			AddedAt:    now,
			Permission: "admin",
		},
	}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/info", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var raw map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshaling body: %v", err)
	}

	if _, exists := raw["permission"]; exists {
		t.Error("expected 'permission' to be absent from wallet info response")
	}
	if _, exists := raw["addedAt"]; exists {
		t.Error("expected 'addedAt' to be absent from wallet info response")
	}
}

// TestGetKeysExcludesUsage ensures usage field is absent from the serialized key response.
func TestGetKeysExcludesUsage(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{
		keys: []wallet.Key{
			{
				ID:    "key-1",
				Kty:   "RSA",
				Usage: []string{"sign_token", "sign_credentials"},
			},
		},
	}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/wallet/keys", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var raw []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatalf("unmarshaling body: %v", err)
	}

	if len(raw) != 1 {
		t.Fatalf("expected 1 key, got %d", len(raw))
	}

	if _, exists := raw[0]["usage"]; exists {
		t.Error("expected 'usage' to be absent from key response")
	}
}

// TestStoreCredential_ContainerAndFlatFields verifies router accepts both container and flat credential requests.
func TestStoreCredential_ContainerAndFlatFields(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"id":"cred-1","participantContextId":"p-1","verifiableCredentialContainer":{"rawVc":"jwt-data","format":"VC1_0_JWT"}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/credential", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if mock.lastImportPlan == nil || mock.lastImportPlan.ID != "cred-1" {
		t.Fatalf("expected lastImportPlan with ID cred-1, got %v", mock.lastImportPlan)
	}
	if mock.lastImportPlan.VerifiableCredentialContainer == nil || mock.lastImportPlan.VerifiableCredentialContainer.RawVc != "jwt-data" {
		t.Errorf("expected container rawVc 'jwt-data', got %v", mock.lastImportPlan.VerifiableCredentialContainer)
	}
}

// TestRequestDcpCredential_IssuerDid verifies issuerDid is mapped from REST request to the domain plan.
func TestRequestDcpCredential_IssuerDid(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"issuerUrl":"https://issuer","issuerDid":"did:web:issuer","holderPid":"super-user","types":["MembershipCredential"],"format":"jwt"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/dcp/request", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusAccepted)
	}
	if mock.lastDcpPlan == nil || mock.lastDcpPlan.IssuerDid != "did:web:issuer" {
		t.Fatalf("expected IssuerDid 'did:web:issuer', got %v", mock.lastDcpPlan)
	}
}

// TestCreateParticipant_WithKeyDescriptor verifies KeyDescriptor is passed through to the domain plan.
func TestCreateParticipant_WithKeyDescriptor(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	body := `{"id":"p-1","did":"did:web:p1","active":true,"key":{"keyId":"k1","privateKeyAlias":"k1-alias","keyGeneratorParams":{"algorithm":"EdDSA","curve":"ed25519"}}}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/wallet/participants", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusCreated)
	}
	if mock.lastParticipantPlan == nil || mock.lastParticipantPlan.KeyDescriptor == nil {
		t.Fatalf("expected KeyDescriptor on participant plan, got %v", mock.lastParticipantPlan)
	}
	if mock.lastParticipantPlan.KeyDescriptor.KeyID != "k1" || mock.lastParticipantPlan.KeyDescriptor.PrivateKeyAlias != "k1-alias" {
		t.Errorf("unexpected KeyDescriptor fields: %+v", mock.lastParticipantPlan.KeyDescriptor)
	}
}

// TestRemoveServiceEndpoint_RouteMapping verifies endpoint ID and DID ID URL parameters are captured.
func TestRemoveServiceEndpoint_RouteMapping(t *testing.T) {
	t.Parallel()

	mock := &mockWallet{}
	engine := setupWalletRouter(mock)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodDelete, "/wallet/did/did:web:super-user/endpoints/ep-1", nil)
	engine.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if mock.lastDidID != "did:web:super-user" || mock.lastEndpointID != "ep-1" {
		t.Errorf("expected did:web:super-user and ep-1, got did=%s, ep=%s", mock.lastDidID, mock.lastEndpointID)
	}
}
