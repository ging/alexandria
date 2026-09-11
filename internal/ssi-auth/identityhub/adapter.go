// Package identityhub adapts Eclipse EDC IdentityHub into the wallet.Wallet domain port.
// It translates domain operations to IdentityHub REST API invocations and handles errors.
package identityhub

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ging/alexandria/internal/common"
	"github.com/ging/alexandria/internal/config"
	"github.com/ging/alexandria/internal/ssi-auth/wallet"
)

var _ wallet.Wallet = (*Adapter)(nil)

// Adapter connects Alexandria domain use cases to an Eclipse EDC IdentityHub instance.
type Adapter struct {
	client *EdcClient
	pid    string
	logger *slog.Logger
}

// New creates an Adapter configured with IdentityHub settings.
func New(cfg *config.IdentityHubConfig, logger *slog.Logger) (*Adapter, error) {
	if cfg == nil {
		return nil, fmt.Errorf("identityhub: configuration cannot be nil")
	}

	if logger == nil {
		logger = slog.Default()
	}

	client, err := NewEdcClient(cfg.IdentityAPIURL, cfg.APIKey, logger)
	if err != nil {
		return nil, err
	}

	pid := cfg.ParticipantID
	if pid == "" {
		pid = "default"
	}

	return &Adapter{
		client: client,
		pid:    pid,
		logger: logger,
	}, nil
}

// Client returns the underlying EdcClient for administrative operations.
func (a *Adapter) Client() *EdcClient {
	return a.client
}

// Close releases the underlying HTTP connection pool.
func (a *Adapter) Close() error {
	return a.client.Close()
}

// getParticipantDids queries all DID documents for the participant context,
// parsing verification methods, public keys, and cryptographic material.
func (a *Adapter) getParticipantDids(ctx context.Context) ([]wallet.Did, error) {
	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return nil, fmt.Errorf("identityhub: getting participant %s: %w", a.pid, err)
	}

	rawDocs, err := a.client.QueryDids(ctx, a.pid)
	if err != nil || len(rawDocs) == 0 {
		return []wallet.Did{normalizeDid(p.ParticipantContextID, p.Did)}, nil
	}

	dids := make([]wallet.Did, 0, len(rawDocs))
	for _, raw := range rawDocs {
		d, err := normalizeDidDoc(p.ParticipantContextID, raw)
		if err != nil {
			dids = append(dids, normalizeDid(p.ParticipantContextID, p.Did))
			continue
		}
		dids = append(dids, d)
	}

	return dids, nil
}

// Link refreshes the active participant identity.
func (a *Adapter) Link(ctx context.Context) (wallet.Did, error) {
	dids, err := a.getParticipantDids(ctx)
	if err != nil {
		return wallet.Did{}, fmt.Errorf("identityhub: linking participant %q: %w", a.pid, err)
	}
	if len(dids) > 0 {
		return dids[0], nil
	}

	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return wallet.Did{}, fmt.Errorf("identityhub: linking participant %q: %w", a.pid, err)
	}

	return normalizeDid(p.ParticipantContextID, p.Did), nil
}

// WalletInfo returns metadata and telemetry for the active wallet participant.
func (a *Adapter) WalletInfo(ctx context.Context) (wallet.WalletInfo, error) {
	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return wallet.WalletInfo{}, err
	}

	dids, err := a.getParticipantDids(ctx)
	if err != nil {
		return wallet.WalletInfo{}, err
	}

	return wallet.WalletInfo{
		ID:         p.ParticipantContextID,
		Name:       "IdentityHub",
		CreatedAt:  p.CreatedAt.Time(),
		Permission: string(p.State),
		Dids:       dids,
	}, nil
}

// RegisterKey imports key material into IdentityHub and returns the registered key.
func (a *Adapter) RegisterKey(ctx context.Context, plan *wallet.KeyPlan) (wallet.Key, error) {
	if plan == nil {
		return wallet.Key{}, fmt.Errorf("identityhub: key plan required: %w", common.ErrInvalidInput)
	}

	alias := plan.Alias
	if alias == "" {
		alias = plan.ID + "-alias"
	}

	var desc KeyDescriptorDto
	if plan.KeyDescriptor != nil {
		keyAlias := alias
		if plan.KeyDescriptor.KeyID != "" && plan.Alias == "" {
			keyAlias = plan.KeyDescriptor.KeyID + "-alias"
		}
		desc = KeyDescriptorDto{
			KeyID:           plan.KeyDescriptor.KeyID,
			Type:            plan.KeyDescriptor.Type,
			PrivateKeyAlias: keyAlias,
			Active:          true,
			KeyContext:      plan.KeyDescriptor.KeyContext,
			Properties:      plan.KeyDescriptor.Properties,
			ResourceURL:     plan.KeyDescriptor.ResourceURL,
		}
	} else {
		desc = KeyDescriptorDto{
			KeyID:           plan.ID,
			PrivateKeyAlias: alias,
			Active:          true,
		}
	}

	if plan.Pem != "" {
		trimmed := strings.TrimSpace(plan.Pem)
		if strings.HasPrefix(trimmed, "{") {
			var jwkMap map[string]any
			if err := json.Unmarshal([]byte(trimmed), &jwkMap); err == nil {
				desc.PublicKeyJwk = jwkMap
				if kid, ok := jwkMap["kid"].(string); ok && kid != "" && (desc.KeyID == "" || strings.HasSuffix(desc.KeyID, ".json")) {
					desc.KeyID = kid
				}
			}
		} else {
			pubPem, err := extractPublicKeyPEM(plan.Pem)
			if err == nil {
				desc.PublicKeyPem = pubPem
			} else {
				desc.PublicKeyPem = plan.Pem
			}
		}
	}

	if err := a.client.AddKey(ctx, a.pid, &desc); err != nil {
		return wallet.Key{}, err
	}

	// Query registered key to return rich domain Key
	keys, err := a.client.ListKeys(ctx, a.pid)
	if err == nil {
		for _, k := range keys {
			if k.KeyID == desc.KeyID || k.PrivateKeyAlias == desc.PrivateKeyAlias {
				return normalizeKey(k), nil
			}
		}
	}

	return normalizeKey(KeyPairDto{
		KeyID:                desc.KeyID,
		ParticipantContextID: a.pid,
		State:                "ACTIVATED",
		PrivateKeyAlias:      desc.PrivateKeyAlias,
		SerializedPublicKey:  desc.PublicKeyPem,
		Descriptor:           &desc,
	}), nil
}

func extractPublicKeyPEM(pemStr string) (string, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return "", fmt.Errorf("no PEM block found")
	}

	if strings.Contains(block.Type, "PUBLIC KEY") {
		return pemStr, nil
	}

	var pub any

	// Try PKCS#8
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		if signer, ok := key.(crypto.Signer); ok {
			pub = signer.Public()
		} else if edKey, ok := key.(ed25519.PrivateKey); ok {
			pub = edKey.Public()
		}
	}

	// Try PKCS#1 (RSA)
	if pub == nil {
		if rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
			pub = &rsaKey.PublicKey
		}
	}

	// Try SEC1 (EC)
	if pub == nil {
		if ecKey, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
			pub = &ecKey.PublicKey
		}
	}

	if pub == nil {
		return "", fmt.Errorf("unable to extract public key from private key")
	}

	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("marshaling public key: %w", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: der,
	})), nil
}

// GetAllKeys retrieves all active key pairs.
func (a *Adapter) GetAllKeys(ctx context.Context) ([]wallet.Key, error) {
	keys, err := a.client.ListKeys(ctx, a.pid)
	if err != nil {
		return nil, err
	}

	return normalizeKeys(keys), nil
}

// resolveKeyPairResourceID maps a user-facing keyID or KeyPair ID to the internal KeyPairResource UUID.
func (a *Adapter) resolveKeyPairResourceID(ctx context.Context, keyID string) (string, error) {
	keys, err := a.client.ListKeys(ctx, a.pid)
	if err != nil {
		return "", fmt.Errorf("identityhub: listing keys: %w", err)
	}

	for _, k := range keys {
		if k.ID == keyID || k.KeyID == keyID || (k.Descriptor != nil && k.Descriptor.KeyID == keyID) || k.PrivateKeyAlias == keyID {
			if k.ID != "" {
				return k.ID, nil
			}
			return k.KeyID, nil
		}
	}

	return "", fmt.Errorf("identityhub: key %q not found for participant %q: %w", keyID, a.pid, common.ErrNotFound)
}

// DeleteKey revokes a key pair in IdentityHub.
func (a *Adapter) DeleteKey(ctx context.Context, keyID string) error {
	resourceID, err := a.resolveKeyPairResourceID(ctx, keyID)
	if err != nil {
		return err
	}
	return a.client.RevokeKey(ctx, a.pid, resourceID)
}

// resolveDid determines the target DID to operate on.
// If didID is empty, matches the participant context ID, or is a friendly alias (not starting with "did:"),
// it resolves to the participant's configured DID in IdentityHub.
// If an explicit DID is specified, it validates that it matches the participant's managed DID.
func (a *Adapter) resolveDid(ctx context.Context, didID string) (string, error) {
	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return "", fmt.Errorf("identityhub: getting participant %s: %w", a.pid, err)
	}

	trimmed := strings.TrimSpace(didID)
	if trimmed == "" || trimmed == a.pid || !strings.HasPrefix(trimmed, "did:") {
		return p.Did, nil
	}

	if trimmed != p.Did {
		return "", fmt.Errorf("identityhub: participant %q only manages DID %q (got %q); to use a new DID, provision a new participant context: %w", a.pid, p.Did, trimmed, common.ErrInvalidInput)
	}

	return p.Did, nil
}

// RegisterDid publishes a DID document to the IdentityHub resolver and returns the minted DID record.
func (a *Adapter) RegisterDid(ctx context.Context, plan *wallet.DidPlan) (wallet.Did, error) {
	if plan == nil {
		return wallet.Did{}, fmt.Errorf("identityhub: did plan required: %w", common.ErrInvalidInput)
	}

	targetDid, err := a.resolveDid(ctx, plan.Alias)
	if err != nil {
		return wallet.Did{}, err
	}

	if plan.Service != nil {
		for _, s := range *plan.Service {
			dto := ServiceEndpointDto{
				ID:              s.ID,
				Type:            string(s.Type),
				ServiceEndpoint: s.Endpoint,
			}
			_ = a.client.AddServiceEndpoint(ctx, a.pid, targetDid, &dto)
		}
	}

	if err := a.client.PublishDid(ctx, a.pid, targetDid); err != nil {
		return wallet.Did{}, err
	}

	return a.GetDidByID(ctx, targetDid)
}

// GetAllDids lists all DIDs associated with the participant.
func (a *Adapter) GetAllDids(ctx context.Context) ([]wallet.Did, error) {
	return a.getParticipantDids(ctx)
}

// GetDidByID retrieves a DID representation by its ID.
func (a *Adapter) GetDidByID(ctx context.Context, didID string) (wallet.Did, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.Did{}, err
	}

	dids, err := a.getParticipantDids(ctx)
	if err != nil {
		return wallet.Did{}, err
	}

	for _, d := range dids {
		if d.ID == targetDid {
			return d, nil
		}
	}

	return normalizeDid(a.pid, targetDid), nil
}

// DeleteDid unpublishes a DID document from the resolver.
func (a *Adapter) DeleteDid(ctx context.Context, didID string) error {
	_, err := a.UnpublishDid(ctx, didID)
	return err
}

// GetAllCredentials collects all verifiable credentials held by the participant.
func (a *Adapter) GetAllCredentials(ctx context.Context) ([]wallet.Credential, error) {
	creds, err := a.client.ListCredentials(ctx, a.pid, "")
	if err != nil {
		return nil, err
	}

	return normalizeCredentials(creds), nil
}

// DeleteCredential removes a stored verifiable credential.
func (a *Adapter) DeleteCredential(ctx context.Context, credentialID string) error {
	return a.client.DeleteCredential(ctx, a.pid, credentialID)
}

// RotateKey triggers key rotation with an overlap window and returns the updated key.
func (a *Adapter) RotateKey(ctx context.Context, keyID string, duration time.Duration) (wallet.Key, error) {
	resourceID, err := a.resolveKeyPairResourceID(ctx, keyID)
	if err != nil {
		return wallet.Key{}, err
	}

	if err := a.client.RotateKey(ctx, a.pid, resourceID, duration); err != nil {
		return wallet.Key{}, err
	}

	keys, err := a.client.ListKeys(ctx, a.pid)
	if err == nil {
		for _, k := range keys {
			if k.ID == resourceID || k.KeyID == keyID {
				return normalizeKey(k), nil
			}
		}
	}

	return wallet.Key{
		ID:        keyID,
		Alias:     keyID,
		Kty:       "RSA",
		State:     "ACTIVATED",
		CreatedAt: time.Now(),
	}, nil
}

// RevokeKey immediately revokes an asymmetric keypair.
func (a *Adapter) RevokeKey(ctx context.Context, keyID string) error {
	resourceID, err := a.resolveKeyPairResourceID(ctx, keyID)
	if err != nil {
		return err
	}
	return a.client.RevokeKey(ctx, a.pid, resourceID)
}

// PublishDid triggers publication of the participant DID document and returns its publication state.
func (a *Adapter) PublishDid(ctx context.Context, didID string) (wallet.DidState, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.DidState{}, err
	}

	p, err := a.client.GetParticipant(ctx, a.pid)
	if err == nil && p.State != "ACTIVATED" {
		_ = a.client.SetParticipantState(ctx, a.pid, true)
	}

	if err := a.client.PublishDid(ctx, a.pid, targetDid); err != nil {
		return wallet.DidState{}, err
	}

	return a.GetDidState(ctx, didID)
}

// UnpublishDid removes publication of the participant DID document and returns its publication state.
func (a *Adapter) UnpublishDid(ctx context.Context, didID string) (wallet.DidState, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.DidState{}, err
	}

	p, err := a.client.GetParticipant(ctx, a.pid)
	if err == nil && p.State == "ACTIVATED" {
		_ = a.client.SetParticipantState(ctx, a.pid, false)
	}

	if err := a.client.UnpublishDid(ctx, a.pid, targetDid); err != nil {
		return wallet.DidState{}, err
	}

	st, err := a.GetDidState(ctx, didID)
	if err != nil {
		return wallet.DidState{
			Did:   targetDid,
			State: "UNPUBLISHED",
		}, nil
	}
	return st, nil
}

// GetDidState inspects DID publication state in the resolver.
func (a *Adapter) GetDidState(ctx context.Context, didID string) (wallet.DidState, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.DidState{}, err
	}

	st, err := a.client.GetDidState(ctx, a.pid, targetDid)
	if err != nil {
		if errors.Is(err, common.ErrNotFound) {
			return wallet.DidState{
				Did:   targetDid,
				State: "UNPUBLISHED",
			}, nil
		}
		return wallet.DidState{}, err
	}

	return wallet.DidState{
		Did:   st.Did,
		State: wallet.DidPublicationState(st.State),
	}, nil
}

// AddServiceEndpoint attaches a new service endpoint to a published DID and returns the updated DID.
func (a *Adapter) AddServiceEndpoint(ctx context.Context, didID string, endpoint wallet.ServiceEndpointPlan) (wallet.Did, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.Did{}, err
	}

	dto := ServiceEndpointDto{
		ID:              endpoint.ID,
		Type:            endpoint.Type,
		ServiceEndpoint: endpoint.URL,
	}

	if err := a.client.AddServiceEndpoint(ctx, a.pid, targetDid, &dto); err != nil {
		return wallet.Did{}, err
	}

	return a.GetDidByID(ctx, didID)
}

// RemoveServiceEndpoint deletes a service endpoint from a published DID and returns the updated DID.
func (a *Adapter) RemoveServiceEndpoint(ctx context.Context, didID string, endpointID string) (wallet.Did, error) {
	targetDid, err := a.resolveDid(ctx, didID)
	if err != nil {
		return wallet.Did{}, err
	}

	if err := a.client.RemoveServiceEndpoint(ctx, a.pid, targetDid, endpointID); err != nil {
		return wallet.Did{}, err
	}

	return a.GetDidByID(ctx, didID)
}

// defaultVerifiableCredentialJSON constructs a minimal valid EDC VerifiableCredential payload.
func defaultVerifiableCredentialJSON(credID, pid string) json.RawMessage {
	return json.RawMessage(fmt.Sprintf(`{"id":%q,"type":["VerifiableCredential"],"issuer":%q,"issuanceDate":%q,"credentialSubject":{"id":%q}}`,
		credID, "did:web:"+pid, time.Now().UTC().Format(time.RFC3339), "did:web:"+pid))
}

// StoreCredential imports an already issued verifiable credential into storage and returns it.
func (a *Adapter) StoreCredential(ctx context.Context, cred *wallet.CredentialImportPlan) (wallet.Credential, error) {
	if cred == nil {
		return wallet.Credential{}, fmt.Errorf("identityhub: credential plan required: %w", common.ErrInvalidInput)
	}

	pid := cred.ParticipantContextID
	if pid == "" {
		pid = a.pid
	}

	rawVc := cred.RawVc
	format := cred.Format
	credObj := cred.Credential

	if cred.VerifiableCredentialContainer != nil {
		if cred.VerifiableCredentialContainer.RawVc != "" {
			rawVc = cred.VerifiableCredentialContainer.RawVc
		}
		if cred.VerifiableCredentialContainer.Format != "" {
			format = cred.VerifiableCredentialContainer.Format
		}
		if len(cred.VerifiableCredentialContainer.Credential) > 0 {
			credObj = cred.VerifiableCredentialContainer.Credential
		}
	}

	if rawVc == "" && len(cred.Payload) > 0 {
		var s string
		if err := json.Unmarshal(cred.Payload, &s); err == nil {
			rawVc = s
		} else {
			rawVc = string(cred.Payload)
		}
	}

	if format == "" {
		format = "VC1_0_JWT"
	}

	if len(credObj) == 0 {
		if json.Valid([]byte(rawVc)) {
			credObj = json.RawMessage(rawVc)
		} else {
			credObj = defaultVerifiableCredentialJSON(cred.ID, pid)
		}
	}

	dto := StoreCredentialDto{
		ID:                   cred.ID,
		ParticipantContextID: pid,
		VerifiableCredentialContainer: VerifiableCredentialContainerDto{
			RawVc:      rawVc,
			Format:     format,
			Credential: credObj,
		},
	}

	if err := a.client.StoreCredential(ctx, a.pid, &dto); err != nil {
		return wallet.Credential{}, err
	}

	creds, err := a.client.ListCredentials(ctx, a.pid, "")
	if err == nil {
		for _, c := range creds {
			if c.ID == cred.ID {
				return normalizeCredential(c), nil
			}
		}
	}

	return normalizeCredential(VerifiableCredentialResourceDto{
		ID:                   cred.ID,
		ParticipantContextID: a.pid,
		VerifiableCredential: VerifiableCredentialBody{
			Format: format,
			RawVc:  rawVc,
		},
	}), nil
}

// GetCredentialsByType queries stored credentials matching a credential type filter.
func (a *Adapter) GetCredentialsByType(ctx context.Context, vcType string) ([]wallet.Credential, error) {
	creds, err := a.client.ListCredentials(ctx, a.pid, vcType)
	if err != nil {
		return nil, err
	}

	return normalizeCredentials(creds), nil
}

// RequestDcpCredential initiates an asynchronous DCP credential acquisition process.
func (a *Adapter) RequestDcpCredential(ctx context.Context, req *wallet.DcpCredentialRequestPlan) (string, error) {
	if req == nil {
		return "", fmt.Errorf("identityhub: dcp request required: %w", common.ErrInvalidInput)
	}

	holderPid := req.HolderPid
	if holderPid == "" {
		holderPid = a.pid
	}

	issuerDid := req.IssuerDid
	if issuerDid == "" {
		issuerDid = req.IssuerURL
	}

	var creds []CredentialDescriptorDto
	if len(req.Credentials) > 0 {
		for _, c := range req.Credentials {
			creds = append(creds, CredentialDescriptorDto{
				ID:     c.ID,
				Format: c.Format,
				Type:   c.Type,
			})
		}
	} else if len(req.Types) > 0 {
		fmtStr := req.Format
		if fmtStr == "" || strings.EqualFold(fmtStr, "jwt") {
			fmtStr = "VC1_0_JWT"
		}
		for i, t := range req.Types {
			creds = append(creds, CredentialDescriptorDto{
				ID:     fmt.Sprintf("req-%d", i+1),
				Format: fmtStr,
				Type:   t,
			})
		}
	}

	dto := DcpCredentialRequestDto{
		IssuerURL:   req.IssuerURL,
		IssuerDid:   issuerDid,
		HolderPid:   holderPid,
		Credentials: creds,
	}

	return a.client.RequestDcpCredential(ctx, a.pid, &dto)
}

// GetDcpRequestStatus checks the state of an in-flight DCP credential request.
func (a *Adapter) GetDcpRequestStatus(ctx context.Context, requestID string) (wallet.DcpRequestStatus, error) {
	st, err := a.client.GetDcpRequestStatus(ctx, a.pid, requestID)
	if err != nil {
		return wallet.DcpRequestStatus{}, err
	}

	return wallet.DcpRequestStatus{
		RequestID: st.RequestID,
		Status:    st.Status,
		Error:     st.Error,
	}, nil
}

// CreateParticipant provisions a new participant context in IdentityHub and returns it.
func (a *Adapter) CreateParticipant(ctx context.Context, plan *wallet.ParticipantPlan) (wallet.Participant, error) {
	if plan == nil {
		return wallet.Participant{}, fmt.Errorf("identityhub: participant plan required: %w", common.ErrInvalidInput)
	}

	var keyDesc *KeyDescriptorDto
	if plan.KeyDescriptor != nil {
		keyID := plan.KeyDescriptor.KeyID
		if keyID == "" {
			keyID = fmt.Sprintf("%s-key-1", plan.ID)
		}
		alias := plan.KeyDescriptor.PrivateKeyAlias
		if alias == "" {
			alias = keyID + "-alias"
		}
		keyDesc = &KeyDescriptorDto{
			KeyID:              keyID,
			Type:               plan.KeyDescriptor.Type,
			PrivateKeyAlias:    alias,
			Active:             true,
			Usage:              []string{"sign_presentation"},
			KeyContext:         plan.KeyDescriptor.KeyContext,
			ResourceURL:        plan.KeyDescriptor.ResourceURL,
			PublicKeyPem:       plan.KeyDescriptor.PublicKeyPem,
			PublicKeyJwk:       plan.KeyDescriptor.PublicKeyJwk,
			KeyGeneratorParams: plan.KeyDescriptor.KeyGeneratorParams,
		}
		if plan.KeyDescriptor.Properties != nil {
			if params, ok := plan.KeyDescriptor.Properties["keyGeneratorParams"].(map[string]any); ok {
				keyDesc.KeyGeneratorParams = params
			} else if _, ok := plan.KeyDescriptor.Properties["algorithm"]; ok {
				keyDesc.KeyGeneratorParams = plan.KeyDescriptor.Properties
			} else {
				keyDesc.Properties = plan.KeyDescriptor.Properties
			}
		}
		if keyDesc.KeyGeneratorParams == nil && keyDesc.PublicKeyPem == "" && keyDesc.PublicKeyJwk == nil {
			keyDesc.KeyGeneratorParams = map[string]any{
				"algorithm": "EdDSA",
				"curve":     "ed25519",
			}
		}
	} else {
		keyID := fmt.Sprintf("%s-key-1", plan.ID)
		keyDesc = &KeyDescriptorDto{
			KeyID:           keyID,
			PrivateKeyAlias: keyID + "-alias",
			Active:          true,
			KeyGeneratorParams: map[string]any{
				"algorithm": "EdDSA",
				"curve":     "ed25519",
			},
		}
	}

	dto := CreateParticipantDto{
		ParticipantContextID: plan.ID,
		Did:                  plan.Did,
		Active:               plan.Active,
		Key:                  keyDesc,
	}

	if err := a.client.CreateParticipant(ctx, &dto); err != nil {
		return wallet.Participant{}, err
	}

	return a.GetParticipant(ctx, plan.ID)
}

// GetParticipant fetches details for a participant context.
func (a *Adapter) GetParticipant(ctx context.Context, participantID string) (wallet.Participant, error) {
	p, err := a.client.GetParticipant(ctx, participantID)
	if err != nil {
		return wallet.Participant{}, err
	}

	return normalizeParticipant(*p), nil
}

// SetParticipantState toggles participant active state and returns the updated participant context.
func (a *Adapter) SetParticipantState(ctx context.Context, participantID string, active bool) (wallet.Participant, error) {
	if err := a.client.SetParticipantState(ctx, participantID, active); err != nil {
		return wallet.Participant{}, err
	}

	return a.GetParticipant(ctx, participantID)
}

// RegenerateParticipantToken rotates the authentication token for a participant.
func (a *Adapter) RegenerateParticipantToken(ctx context.Context, participantID string) (string, error) {
	tok, err := a.client.RegenerateParticipantToken(ctx, participantID)
	if err != nil {
		return "", err
	}

	if participantID == a.pid {
		a.client.SetAPIKey(tok)
		a.logger.WarnContext(ctx, "regenerated API token for active participant context; updated in-memory client. Please update wallet_config.api_key in configuration before restarting",
			"participant_id", participantID)
	}

	return tok, nil
}

// UpdateParticipantToken hot-updates the in-memory authentication token for the active participant.
func (a *Adapter) UpdateParticipantToken(ctx context.Context, participantID string, token string) error {
	if participantID == a.pid {
		a.client.SetAPIKey(token)
		a.logger.InfoContext(ctx, "updated in-memory API token for active participant",
			"participant_id", participantID)
		return nil
	}
	return nil
}

// ===== UNSUPPORTED OPERATIONS IN IDENTITYHUB =================================

// SetDefaultDid is not supported in IdentityHub.
func (a *Adapter) SetDefaultDid(_ context.Context, _ string) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInIdentityHub
}

// AddKeyToDid is not supported in IdentityHub.
func (a *Adapter) AddKeyToDid(_ context.Context, _, _ string) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInIdentityHub
}

// RemoveKeyFromDid is not supported in IdentityHub.
func (a *Adapter) RemoveKeyFromDid(_ context.Context, _, _ string) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInIdentityHub
}

// SetDefaultKey is not supported in IdentityHub.
func (a *Adapter) SetDefaultKey(_ context.Context, _, _ string) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInIdentityHub
}

// ProcessOid4vci is not supported in IdentityHub.
func (a *Adapter) ProcessOid4vci(_ context.Context, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// ProcessOid4vp is not supported in IdentityHub.
func (a *Adapter) ProcessOid4vp(_ context.Context, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}
