// Service implements the core wallet business logic and identity use cases.
// It caches active identity state and orchestrates operations through wallet ports.

package wallet

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/common"
	"github.com/trustbloc/did-go/doc/did"
)

// Service is the wallet use case. It owns the rules — what key material is
// acceptable, what a DID needs before it can be minted — and delegates
// everything else through the ports.
//
// The active identity is cached behind a mutex because it is read on every
// request that needs to sign and written only when the wallet is relinked.
type Service struct {
	wallet       Wallet
	pemInspector PemInspector
	clock        Clock
	logger       *slog.Logger
	mu           sync.RWMutex
	identity     *Did
}

// NewService wires the use case onto its ports. A nil logger falls back to the
// default one; the ports themselves are required.
func NewService(
	wallet Wallet,
	pemInspector PemInspector,
	clock Clock,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		wallet:       wallet,
		pemInspector: pemInspector,
		clock:        clock,
		logger:       logger,
		mu:           sync.RWMutex{},
		identity:     nil,
	}
}

// ===== Linking ===============================================================

// Link refreshes the wallet identity from the outsourced wallet.
func (s *Service) Link(ctx context.Context) (Did, error) {
	did, err := s.wallet.Link(ctx)
	if err != nil {
		return Did{}, fmt.Errorf("linking wallet: %w", err)
	}

	if did.ID == "" {
		return Did{}, fmt.Errorf("wallet reported no default did: %w", common.ErrNotLinked)
	}
	s.setIdentity(did)
	s.logger.InfoContext(ctx, "identity established", "did", did.ID, "alias", did.Alias)

	return did, nil
}

// IsLinked reports whether the wallet is registered in the directory.
func (s *Service) IsLinked(_ context.Context) bool {
	_, ok := s.identitySnapshot()
	return ok
}

// ===== Keys ==================================================================

// RegisterKey imports raw PEM or JOSE key material and indexes it under an optional alias.
func (s *Service) RegisterKey(ctx context.Context, pem string, alias *string, id *string) (Key, error) {
	var a string
	if alias != nil {
		a = *alias
	}

	pemDescriptor, err := s.pemInspector.Inspect(pem)
	if err != nil {
		return Key{}, common.Invalid("pem", err.Error())
	}

	isJSON := strings.HasPrefix(strings.TrimSpace(pem), "{")
	if !isJSON && !pemDescriptor.Private {
		return Key{}, common.Invalid("pem", "carries only a public key; the wallet has to be able to sign with it")
	}

	keyID := fmt.Sprintf("%s.json", pemDescriptor.Thumbprint)
	if id != nil {
		if strings.TrimSpace(*id) == "" {
			return Key{}, common.Invalid("id", "is present but empty; omit it to have the key named after its thumbprint")
		}

		keyID = *id
	}

	keyPlan := &KeyPlan{
		ID:    keyID,
		Alias: a,
		Pem:   pem,
	}

	key, err := s.wallet.RegisterKey(ctx, keyPlan)
	if err != nil {
		return Key{}, fmt.Errorf("wallet reported error by key registering: %w", err)
	}

	s.logger.InfoContext(ctx, "key registered",
		"id", keyID, "kid", pemDescriptor.Thumbprint,
		"kty", pemDescriptor.Kty, "alias", a)

	return key, nil
}

// DeleteKey purges a key, provided no DID still references it.
func (s *Service) DeleteKey(c context.Context, keyID string) error {
	err := s.wallet.DeleteKey(c, keyID)
	if err != nil {
		return fmt.Errorf("wallet reported error by deleting keys: %w", err)
	}
	return nil
}

// Keys lists every keypair held by the wallet.
func (s *Service) Keys(c context.Context) ([]Key, error) {
	keys, err := s.wallet.GetAllKeys(c)
	if err != nil {
		return []Key{}, fmt.Errorf("wallet reported error by fetching keys: %w", err)
	}
	return keys, nil
}

// ===== DIDs ==================================================================

// RegisterDid mints a local DID from the given builder, binding the referenced
// keys as verification methods, and persists it.
func (s *Service) RegisterDid(
	ctx context.Context,
	builder common.DidBuilder,
	keys []string,
	alias string,
	services []common.DidService,
) (Did, error) {
	if err := builder.Validate(); err != nil {
		return Did{}, fmt.Errorf("wallet reported error by did registering: %w", err)
	}

	for _, k := range keys {
		if strings.TrimSpace(k) == "" {
			return Did{}, common.Invalid("keys", "carries an empty key id")
		}
	}

	didPlan := &DidPlan{
		Builder: builder,
		Alias:   alias,
		Keys:    keys,
		Service: &services,
	}

	did, err := s.wallet.RegisterDid(ctx, didPlan)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by did registering: %w", err)
	}

	s.adopt(did)

	s.logger.InfoContext(ctx, "did registered",
		"method", builder.Method(), "keys", keys, "id", did.ID)

	return did, nil
}

// Did resolves the identifier of the wallet default DID.
func (s *Service) Did(_ context.Context) (string, error) {
	id, ok := s.identitySnapshot()
	if !ok {
		return "", common.ErrNotLinked
	}
	return id.Document.ID, nil
}

// DidDoc resolves the DID Document of the default DID, as served publicly.
func (s *Service) DidDoc(_ context.Context) (did.Doc, error) {
	identity, ok := s.identitySnapshot()
	if !ok {
		return did.Doc{}, common.ErrNotLinked
	}

	return identity.Document, nil
}

// GetAllDids gets all dids from wallet
func (s *Service) GetAllDids(c context.Context) ([]Did, error) {
	dids, err := s.wallet.GetAllDids(c)
	if err != nil {
		return []Did{}, fmt.Errorf("wallet reported error by getting dids: %w", err)
	}
	return dids, nil
}

// GetDidByID gets did by ID from wallet
func (s *Service) GetDidByID(c context.Context, didID string) (Did, error) {
	did, err := s.wallet.GetDidByID(c, didID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by getting did: %w", err)
	}
	return did, nil
}

// DeleteDid drops a DID and its verification method bindings.
func (s *Service) DeleteDid(c context.Context, didID string) error {
	err := s.wallet.DeleteDid(c, didID)
	if err != nil {
		return fmt.Errorf("wallet reported error by deleting dids: %w", err)
	}
	return nil
}

// SetDefaultDid promotes a DID to be the wallet primary identity and returns it.
func (s *Service) SetDefaultDid(c context.Context, didID string) (Did, error) {
	did, err := s.wallet.SetDefaultDid(c, didID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by setting did as default: %w", err)
	}
	s.adopt(did)
	return did, nil
}

// ===== DID verification methods ==============================================

// AddKeyToDid binds a key into the verification methods of a DID and returns the updated DID.
func (s *Service) AddKeyToDid(c context.Context, didID, keyID string) (Did, error) {
	did, err := s.wallet.AddKeyToDid(c, didID, keyID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by adding key from did: %w", err)
	}
	s.adopt(did)
	return did, nil
}

// RemoveKeyFromDid unbinds a key from the verification methods of a DID and returns the updated DID.
func (s *Service) RemoveKeyFromDid(c context.Context, didID, keyID string) (Did, error) {
	did, err := s.wallet.RemoveKeyFromDid(c, didID, keyID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by removing key from did: %w", err)
	}
	s.adopt(did)
	return did, nil
}

// SetDefaultKey promotes a key to be the default verification method of a DID and returns the updated DID.
func (s *Service) SetDefaultKey(c context.Context, didID, keyID string) (Did, error) {
	did, err := s.wallet.SetDefaultKey(c, didID, keyID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error by setting key as default: %w", err)
	}
	s.adopt(did)
	return did, nil
}

// ===== Credentials ===========================================================

// DeleteCredential purges a stored Verifiable Credential.
func (s *Service) DeleteCredential(c context.Context, credentialID string) error {
	err := s.wallet.DeleteCredential(c, credentialID)
	if err != nil {
		return fmt.Errorf("wallet reported error by deleting credential: %w", err)
	}
	return nil
}

// Credentials lists every Verifiable Credential held by the wallet.
func (s *Service) Credentials(c context.Context) ([]Credential, error) {
	credentials, err := s.wallet.GetAllCredentials(c)
	if err != nil {
		return []Credential{}, fmt.Errorf("wallet reported error by fetching credentials: %w", err)
	}
	return credentials, nil
}

// ===== Runtime state =========================================================

// Info reports the wallet runtime state.
func (s *Service) Info(c context.Context) (WalletInfo, error) {
	walletInfo, err := s.wallet.WalletInfo(c)
	if err != nil {
		return WalletInfo{}, fmt.Errorf("wallet reported error by getting wallet info: %w", err)
	}
	return walletInfo, nil
}

// ===== OpenID4VC =============================================================

// ProcessOid4vci accepts an inbound OID4VCI credential offer and stores the
// credential it yields.
func (s *Service) ProcessOid4vci(ctx context.Context, uri string) error {
	if strings.TrimSpace(uri) == "" {
		return common.Invalid("uri", "is required")
	}

	if err := s.wallet.ProcessOid4vci(ctx, uri); err != nil {
		return fmt.Errorf("wallet reported error by processing oid4vci: %w", err)
	}

	s.logger.InfoContext(ctx, "oid4vci processed", "uri", uri)
	return nil
}

// ProcessOid4vp answers an outbound OID4VP presentation request.
func (s *Service) ProcessOid4vp(ctx context.Context, uri string) error {
	if strings.TrimSpace(uri) == "" {
		return common.Invalid("uri", "is required")
	}

	if err := s.wallet.ProcessOid4vp(ctx, uri); err != nil {
		return fmt.Errorf("wallet reported error by processing oid4vp: %w", err)
	}

	s.logger.InfoContext(ctx, "oid4vp processed", "uri", uri)
	return nil
}

// RotateKey requests the wallet to rotate an existing key and returns the updated key.
func (s *Service) RotateKey(ctx context.Context, keyID string, duration time.Duration) (Key, error) {
	if strings.TrimSpace(keyID) == "" {
		return Key{}, common.Invalid("id", "is required")
	}

	key, err := s.wallet.RotateKey(ctx, keyID, duration)
	if err != nil {
		return Key{}, fmt.Errorf("wallet reported error by rotating key: %w", err)
	}

	return key, nil
}

// RevokeKey requests the wallet to revoke an existing key.
func (s *Service) RevokeKey(ctx context.Context, keyID string) error {
	if strings.TrimSpace(keyID) == "" {
		return common.Invalid("id", "is required")
	}

	if err := s.wallet.RevokeKey(ctx, keyID); err != nil {
		return fmt.Errorf("wallet reported error by revoking key: %w", err)
	}

	return nil
}

// PublishDid requests the wallet to publish a DID document and returns its publication state.
func (s *Service) PublishDid(ctx context.Context, didID string) (DidState, error) {
	if strings.TrimSpace(didID) == "" {
		return DidState{}, common.Invalid("id", "is required")
	}

	st, err := s.wallet.PublishDid(ctx, didID)
	if err != nil {
		return DidState{}, fmt.Errorf("wallet reported error by publishing did: %w", err)
	}

	return st, nil
}

// UnpublishDid requests the wallet to unpublish a DID document and returns its publication state.
func (s *Service) UnpublishDid(ctx context.Context, didID string) (DidState, error) {
	if strings.TrimSpace(didID) == "" {
		return DidState{}, common.Invalid("id", "is required")
	}

	st, err := s.wallet.UnpublishDid(ctx, didID)
	if err != nil {
		return DidState{}, fmt.Errorf("wallet reported error by unpublishing did: %w", err)
	}

	return st, nil
}

// GetDidState queries the publication state of a DID.
func (s *Service) GetDidState(ctx context.Context, didID string) (DidState, error) {
	if strings.TrimSpace(didID) == "" {
		return DidState{}, common.Invalid("id", "is required")
	}

	st, err := s.wallet.GetDidState(ctx, didID)
	if err != nil {
		return DidState{}, fmt.Errorf("wallet reported error getting did state: %w", err)
	}

	return st, nil
}

// AddServiceEndpoint adds an endpoint to a DID document and returns the updated DID.
func (s *Service) AddServiceEndpoint(ctx context.Context, didID string, endpoint ServiceEndpointPlan) (Did, error) {
	if strings.TrimSpace(didID) == "" {
		return Did{}, common.Invalid("id", "is required")
	}
	if strings.TrimSpace(endpoint.ID) == "" {
		return Did{}, common.Invalid("endpoint.id", "is required")
	}

	did, err := s.wallet.AddServiceEndpoint(ctx, didID, endpoint)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error adding service endpoint: %w", err)
	}

	s.adopt(did)
	return did, nil
}

// RemoveServiceEndpoint deletes an endpoint from a DID document and returns the updated DID.
func (s *Service) RemoveServiceEndpoint(ctx context.Context, didID, endpointID string) (Did, error) {
	if strings.TrimSpace(didID) == "" {
		return Did{}, common.Invalid("id", "is required")
	}
	if strings.TrimSpace(endpointID) == "" {
		return Did{}, common.Invalid("endpoint_id", "is required")
	}

	did, err := s.wallet.RemoveServiceEndpoint(ctx, didID, endpointID)
	if err != nil {
		return Did{}, fmt.Errorf("wallet reported error removing service endpoint: %w", err)
	}

	s.adopt(did)
	return did, nil
}

// StoreCredential imports a verifiable credential directly into wallet storage and returns it.
func (s *Service) StoreCredential(ctx context.Context, cred *CredentialImportPlan) (Credential, error) {
	if cred == nil {
		return Credential{}, common.Invalid("credential", "is required")
	}

	c, err := s.wallet.StoreCredential(ctx, cred)
	if err != nil {
		return Credential{}, fmt.Errorf("wallet reported error storing credential: %w", err)
	}

	return c, nil
}

// GetCredentialsByType queries credentials filtered by type.
func (s *Service) GetCredentialsByType(ctx context.Context, vcType string) ([]Credential, error) {
	creds, err := s.wallet.GetCredentialsByType(ctx, vcType)
	if err != nil {
		return nil, fmt.Errorf("wallet reported error querying credentials: %w", err)
	}

	return creds, nil
}

// RequestDcpCredential requests a verifiable credential through DCP.
func (s *Service) RequestDcpCredential(ctx context.Context, req *DcpCredentialRequestPlan) (string, error) {
	if req == nil {
		return "", common.Invalid("request", "is required")
	}

	reqID, err := s.wallet.RequestDcpCredential(ctx, req)
	if err != nil {
		return "", fmt.Errorf("wallet reported error requesting dcp credential: %w", err)
	}

	return reqID, nil
}

// GetDcpRequestStatus inspects the status of an ongoing DCP request.
func (s *Service) GetDcpRequestStatus(ctx context.Context, requestID string) (DcpRequestStatus, error) {
	if strings.TrimSpace(requestID) == "" {
		return DcpRequestStatus{}, common.Invalid("id", "is required")
	}

	st, err := s.wallet.GetDcpRequestStatus(ctx, requestID)
	if err != nil {
		return DcpRequestStatus{}, fmt.Errorf("wallet reported error getting dcp status: %w", err)
	}

	return st, nil
}

// CreateParticipant creates a new participant context in the wallet and returns it.
func (s *Service) CreateParticipant(ctx context.Context, plan *ParticipantPlan) (Participant, error) {
	if plan == nil {
		return Participant{}, common.Invalid("participant", "is required")
	}

	p, err := s.wallet.CreateParticipant(ctx, plan)
	if err != nil {
		return Participant{}, fmt.Errorf("wallet reported error creating participant: %w", err)
	}

	return p, nil
}

// GetParticipant fetches a participant context from the wallet.
func (s *Service) GetParticipant(ctx context.Context, participantID string) (Participant, error) {
	if strings.TrimSpace(participantID) == "" {
		return Participant{}, common.Invalid("id", "is required")
	}

	p, err := s.wallet.GetParticipant(ctx, participantID)
	if err != nil {
		return Participant{}, fmt.Errorf("wallet reported error getting participant: %w", err)
	}

	return p, nil
}

// SetParticipantState toggles active status for a participant context and returns it.
func (s *Service) SetParticipantState(ctx context.Context, participantID string, active bool) (Participant, error) {
	if strings.TrimSpace(participantID) == "" {
		return Participant{}, common.Invalid("id", "is required")
	}

	p, err := s.wallet.SetParticipantState(ctx, participantID, active)
	if err != nil {
		return Participant{}, fmt.Errorf("wallet reported error setting participant state: %w", err)
	}

	return p, nil
}

// RegenerateParticipantToken rotates the token for a participant context.
func (s *Service) RegenerateParticipantToken(ctx context.Context, participantID string) (string, error) {
	if strings.TrimSpace(participantID) == "" {
		return "", common.Invalid("id", "is required")
	}

	tok, err := s.wallet.RegenerateParticipantToken(ctx, participantID)
	if err != nil {
		return "", fmt.Errorf("wallet reported error regenerating token: %w", err)
	}

	return tok, nil
}

// UpdateParticipantToken updates the in-memory token for a participant context.
func (s *Service) UpdateParticipantToken(ctx context.Context, participantID string, token string) error {
	if strings.TrimSpace(participantID) == "" {
		return common.Invalid("id", "is required")
	}
	if strings.TrimSpace(token) == "" {
		return common.Invalid("token", "is required")
	}

	if err := s.wallet.UpdateParticipantToken(ctx, participantID, token); err != nil {
		return fmt.Errorf("wallet reported error updating token: %w", err)
	}

	return nil
}

// =============================================================
// Accesors
// =============================================================

// setIdentity replaces the active identity.
func (s *Service) setIdentity(d Did) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.identity = &d
}

// adopt replaces the active identity only if the wallet promoted this DID
// to default. Mutations funnel through here.
func (s *Service) adopt(d Did) {
	if d.Default {
		s.setIdentity(d)
	}
}

// identitySnapshot returns the active identity, or false if not linked yet.
func (s *Service) identitySnapshot() (Did, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.identity == nil {
		return Did{}, false
	}
	return *s.identity, true
}
