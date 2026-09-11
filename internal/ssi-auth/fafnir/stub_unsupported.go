// stub_unsupported stubs operations not supported by the Fafnir wallet implementation.
// It returns domain sentinels indicating unsupported capabilities.

package fafnir

import (
	"context"
	"time"

	"github.com/ging/alexandria/internal/common"
	"github.com/ging/alexandria/internal/ssi-auth/wallet"
)

// RotateKey returns not implemented for Fafnir.
func (a *Adapter) RotateKey(_ context.Context, _ string, _ time.Duration) (wallet.Key, error) {
	return wallet.Key{}, common.ErrNotImplementedInFafnir
}

// RevokeKey returns not implemented for Fafnir.
func (a *Adapter) RevokeKey(_ context.Context, _ string) error {
	return common.ErrNotImplementedInFafnir
}

// PublishDid returns not implemented for Fafnir.
func (a *Adapter) PublishDid(_ context.Context, _ string) (wallet.DidState, error) {
	return wallet.DidState{}, common.ErrNotImplementedInFafnir
}

// UnpublishDid returns not implemented for Fafnir.
func (a *Adapter) UnpublishDid(_ context.Context, _ string) (wallet.DidState, error) {
	return wallet.DidState{}, common.ErrNotImplementedInFafnir
}

// GetDidState returns not implemented for Fafnir.
func (a *Adapter) GetDidState(_ context.Context, _ string) (wallet.DidState, error) {
	return wallet.DidState{}, common.ErrNotImplementedInFafnir
}

// AddServiceEndpoint returns not implemented for Fafnir.
func (a *Adapter) AddServiceEndpoint(_ context.Context, _ string, _ wallet.ServiceEndpointPlan) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInFafnir
}

// RemoveServiceEndpoint returns not implemented for Fafnir.
func (a *Adapter) RemoveServiceEndpoint(_ context.Context, _ string, _ string) (wallet.Did, error) {
	return wallet.Did{}, common.ErrNotImplementedInFafnir
}

// StoreCredential returns not implemented for Fafnir.
func (a *Adapter) StoreCredential(_ context.Context, _ *wallet.CredentialImportPlan) (wallet.Credential, error) {
	return wallet.Credential{}, common.ErrNotImplementedInFafnir
}

// GetCredentialsByType returns not implemented for Fafnir.
func (a *Adapter) GetCredentialsByType(_ context.Context, _ string) ([]wallet.Credential, error) {
	return nil, common.ErrNotImplementedInFafnir
}

// RequestDcpCredential returns not implemented for Fafnir.
func (a *Adapter) RequestDcpCredential(_ context.Context, _ *wallet.DcpCredentialRequestPlan) (string, error) {
	return "", common.ErrNotImplementedInFafnir
}

// GetDcpRequestStatus returns not implemented for Fafnir.
func (a *Adapter) GetDcpRequestStatus(_ context.Context, _ string) (wallet.DcpRequestStatus, error) {
	return wallet.DcpRequestStatus{}, common.ErrNotImplementedInFafnir
}

// CreateParticipant returns not implemented for Fafnir.
func (a *Adapter) CreateParticipant(_ context.Context, _ *wallet.ParticipantPlan) (wallet.Participant, error) {
	return wallet.Participant{}, common.ErrNotImplementedInFafnir
}

// GetParticipant returns not implemented for Fafnir.
func (a *Adapter) GetParticipant(_ context.Context, _ string) (wallet.Participant, error) {
	return wallet.Participant{}, common.ErrNotImplementedInFafnir
}

// SetParticipantState returns not implemented for Fafnir.
func (a *Adapter) SetParticipantState(_ context.Context, _ string, _ bool) (wallet.Participant, error) {
	return wallet.Participant{}, common.ErrNotImplementedInFafnir
}

// RegenerateParticipantToken returns not implemented for Fafnir.
func (a *Adapter) RegenerateParticipantToken(_ context.Context, _ string) (string, error) {
	return "", common.ErrNotImplementedInFafnir
}

// UpdateParticipantToken returns not implemented for Fafnir.
func (a *Adapter) UpdateParticipantToken(_ context.Context, _ string, _ string) error {
	return common.ErrNotImplementedInFafnir
}
