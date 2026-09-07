// Package identityhub adapts Eclipse EDC IdentityHub into the wallet.Wallet domain port.
// It translates domain operations to IdentityHub REST API invocations and handles errors.
package identityhub

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/config"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
)

var _ wallet.Wallet = (*Adapter)(nil)

// Adapter connects Alexandria domain use cases to an Eclipse EDC IdentityHub instance.
type Adapter struct {
	client *Client
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

	client, err := NewClient(cfg.IdentityAPIURL, cfg.APIKey, logger)
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

// Close releases the underlying HTTP connection pool.
func (a *Adapter) Close() error {
	return a.client.Close()
}

// Link refreshes the active participant identity.
func (a *Adapter) Link(ctx context.Context) (wallet.Did, error) {
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

	return wallet.WalletInfo{
		ID:         p.ParticipantContextID,
		Name:       "IdentityHub",
		CreatedAt:  p.CreatedAt.Time(),
		Permission: string(p.State),
		Dids:       []wallet.Did{normalizeDid(p.ParticipantContextID, p.Did)},
	}, nil
}

// RegisterKey imports key material into IdentityHub.
func (a *Adapter) RegisterKey(ctx context.Context, plan *wallet.KeyPlan) error {
	if plan == nil {
		return fmt.Errorf("identityhub: key plan required: %w", common.ErrInvalidInput)
	}

	var desc KeyDescriptorDto
	if plan.KeyDescriptor != nil {
		desc = KeyDescriptorDto{
			KeyID:       plan.KeyDescriptor.KeyID,
			Type:        plan.KeyDescriptor.Type,
			KeyContext:  plan.KeyDescriptor.KeyContext,
			Properties:  plan.KeyDescriptor.Properties,
			ResourceURL: plan.KeyDescriptor.ResourceURL,
		}
	} else {
		desc = KeyDescriptorDto{
			KeyID: plan.ID,
			Type:  "OKP",
			Properties: map[string]any{
				"pem": plan.Pem,
			},
		}
	}

	return a.client.AddKey(ctx, a.pid, &desc)
}

// GetAllKeys retrieves all active key pairs.
func (a *Adapter) GetAllKeys(ctx context.Context) ([]wallet.Key, error) {
	keys, err := a.client.ListKeys(ctx, a.pid)
	if err != nil {
		return nil, err
	}

	return normalizeKeys(keys), nil
}

// DeleteKey revokes a key pair in IdentityHub.
func (a *Adapter) DeleteKey(ctx context.Context, keyID string) error {
	return a.client.RevokeKey(ctx, a.pid, keyID)
}

// RegisterDid publishes a DID document to the IdentityHub resolver.
func (a *Adapter) RegisterDid(ctx context.Context, plan *wallet.DidPlan) error {
	if plan == nil {
		return fmt.Errorf("identityhub: did plan required: %w", common.ErrInvalidInput)
	}

	return a.client.PublishDid(ctx, a.pid, plan.Alias)
}

// GetAllDids lists all DIDs associated with the participant.
func (a *Adapter) GetAllDids(ctx context.Context) ([]wallet.Did, error) {
	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return nil, err
	}

	return []wallet.Did{normalizeDid(p.ParticipantContextID, p.Did)}, nil
}

// GetDidByID retrieves a DID representation by its ID.
func (a *Adapter) GetDidByID(ctx context.Context, didID string) (wallet.Did, error) {
	p, err := a.client.GetParticipant(ctx, a.pid)
	if err != nil {
		return wallet.Did{}, err
	}

	return normalizeDid(p.ParticipantContextID, didID), nil
}

// DeleteDid unpublishes a DID document from the resolver.
func (a *Adapter) DeleteDid(ctx context.Context, didID string) error {
	return a.client.UnpublishDid(ctx, a.pid, didID)
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

// RotateKey triggers key rotation with an overlap window.
func (a *Adapter) RotateKey(ctx context.Context, keyID string, duration time.Duration) error {
	return a.client.RotateKey(ctx, a.pid, keyID, duration)
}

// RevokeKey immediately revokes an asymmetric keypair.
func (a *Adapter) RevokeKey(ctx context.Context, keyID string) error {
	return a.client.RevokeKey(ctx, a.pid, keyID)
}

// PublishDid triggers publication of the participant DID document.
func (a *Adapter) PublishDid(ctx context.Context, didID string) error {
	return a.client.PublishDid(ctx, a.pid, didID)
}

// UnpublishDid removes publication of the participant DID document.
func (a *Adapter) UnpublishDid(ctx context.Context, didID string) error {
	return a.client.UnpublishDid(ctx, a.pid, didID)
}

// GetDidState inspects DID publication state in the resolver.
func (a *Adapter) GetDidState(ctx context.Context, didID string) (wallet.DidState, error) {
	st, err := a.client.GetDidState(ctx, a.pid, didID)
	if err != nil {
		return wallet.DidState{}, err
	}

	return wallet.DidState{
		Did:   st.Did,
		State: wallet.DidPublicationState(st.State),
	}, nil
}

// AddServiceEndpoint attaches a new service endpoint to a published DID.
func (a *Adapter) AddServiceEndpoint(ctx context.Context, _ string, endpoint wallet.ServiceEndpointPlan) error {
	dto := ServiceEndpointDto{
		ID:              endpoint.ID,
		Type:            endpoint.Type,
		ServiceEndpoint: endpoint.URL,
	}

	return a.client.AddServiceEndpoint(ctx, a.pid, &dto)
}

// RemoveServiceEndpoint deletes a service endpoint from a published DID.
func (a *Adapter) RemoveServiceEndpoint(ctx context.Context, _ string, endpointID string) error {
	return a.client.RemoveServiceEndpoint(ctx, a.pid, endpointID)
}

// StoreCredential imports an already issued verifiable credential into storage.
func (a *Adapter) StoreCredential(ctx context.Context, cred *wallet.CredentialImportPlan) error {
	if cred == nil {
		return fmt.Errorf("identityhub: credential plan required: %w", common.ErrInvalidInput)
	}

	dto := StoreCredentialDto{
		ID:    cred.ID,
		RawVc: string(cred.Payload),
	}

	return a.client.StoreCredential(ctx, a.pid, &dto)
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

	dto := DcpCredentialRequestDto{
		IssuerURL: req.IssuerURL,
		HolderPid: req.HolderPid,
		Types:     req.Types,
		Format:    req.Format,
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

// CreateParticipant provisions a new participant context in IdentityHub.
func (a *Adapter) CreateParticipant(ctx context.Context, plan *wallet.ParticipantPlan) error {
	if plan == nil {
		return fmt.Errorf("identityhub: participant plan required: %w", common.ErrInvalidInput)
	}

	var keyDesc *KeyDescriptorDto
	if plan.KeyDescriptor != nil {
		alias := plan.KeyDescriptor.KeyID + "-alias"
		keyDesc = &KeyDescriptorDto{
			KeyID:           plan.KeyDescriptor.KeyID,
			Type:            plan.KeyDescriptor.Type,
			PrivateKeyAlias: alias,
			Active:          true,
			Usage:           []string{"sign_presentation"},
			KeyContext:      plan.KeyDescriptor.KeyContext,
			ResourceURL:     plan.KeyDescriptor.ResourceURL,
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
	}

	dto := CreateParticipantDto{
		ParticipantContextID: plan.ID,
		Did:                  plan.Did,
		Active:               plan.Active,
		Key:                  keyDesc,
	}

	return a.client.CreateParticipant(ctx, &dto)
}

// GetParticipant fetches details for a participant context.
func (a *Adapter) GetParticipant(ctx context.Context, participantID string) (wallet.Participant, error) {
	p, err := a.client.GetParticipant(ctx, participantID)
	if err != nil {
		return wallet.Participant{}, err
	}

	return normalizeParticipant(*p), nil
}

// SetParticipantState toggles participant active state.
func (a *Adapter) SetParticipantState(ctx context.Context, participantID string, active bool) error {
	return a.client.SetParticipantState(ctx, participantID, active)
}

// RegenerateParticipantToken rotates the authentication token for a participant.
func (a *Adapter) RegenerateParticipantToken(ctx context.Context, participantID string) (string, error) {
	return a.client.RegenerateParticipantToken(ctx, participantID)
}

// ===== UNSUPPORTED OPERATIONS IN IDENTITYHUB =================================

// SetDefaultDid is not supported in IdentityHub.
func (a *Adapter) SetDefaultDid(_ context.Context, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// AddKeyToDid is not supported in IdentityHub.
func (a *Adapter) AddKeyToDid(_ context.Context, _, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// RemoveKeyFromDid is not supported in IdentityHub.
func (a *Adapter) RemoveKeyFromDid(_ context.Context, _, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// SetDefaultKey is not supported in IdentityHub.
func (a *Adapter) SetDefaultKey(_ context.Context, _, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// ProcessOid4vci is not supported in IdentityHub.
func (a *Adapter) ProcessOid4vci(_ context.Context, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}

// ProcessOid4vp is not supported in IdentityHub.
func (a *Adapter) ProcessOid4vp(_ context.Context, _ string) error {
	return common.ErrNotImplementedInIdentityHub
}
