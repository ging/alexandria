// Package wallet defines the outbound driven interfaces required by the wallet domain.
// It decouples wallet business logic from external transports and key stores.
package wallet

import (
	"context"
	"time"
)

// ===== Wallet driven adaprter port ===========================================

// Wallet is the outsourced wallet holding the key material and the DIDs.
type Wallet interface {
	// Link refreshes the wallet identity, returning whatever the default DID
	Link(ctx context.Context) (Did, error)
	// RegisterKey files raw key material and returns the registered key.
	RegisterKey(ctx context.Context, keyPlan *KeyPlan) (Key, error)
	// Get all keys
	GetAllKeys(ctx context.Context) ([]Key, error)
	// Delete key
	DeleteKey(ctx context.Context, keyID string) error
	// Registers did and returns the minted DID record.
	RegisterDid(ctx context.Context, didPlan *DidPlan) (Did, error)
	// GetAllDids
	GetAllDids(ctx context.Context) ([]Did, error)
	// GetDidByID
	GetDidByID(ctx context.Context, didID string) (Did, error)
	// Delete did
	DeleteDid(ctx context.Context, didID string) error
	// SetDefaultDid promotes a DID and returns the updated default DID.
	SetDefaultDid(ctx context.Context, didID string) (Did, error)
	// AddKeyToDid binds a key to a DID and returns the updated DID.
	AddKeyToDid(ctx context.Context, didID string, keyID string) (Did, error)
	// RemoveKeyFromDid unbinds a key from a DID and returns the updated DID.
	RemoveKeyFromDid(ctx context.Context, didID string, keyID string) (Did, error)
	// SetDefaultKey promotes a key to default on a DID and returns the updated DID.
	SetDefaultKey(ctx context.Context, didID string, keyID string) (Did, error)
	// WalletInfo
	WalletInfo(ctx context.Context) (WalletInfo, error)
	// GetAllCredentials lists every Verifiable Credential held by the wallet.
	GetAllCredentials(ctx context.Context) ([]Credential, error)
	// DeleteCredential purges a stored Verifiable Credential.
	DeleteCredential(ctx context.Context, credentialID string) error
	// ProcessOid4vci processes an inbound OID4VCI credential offer URI.
	ProcessOid4vci(ctx context.Context, uri string) error
	// ProcessOid4vp processes an outbound OID4VP presentation request URI.
	ProcessOid4vp(ctx context.Context, uri string) error
	// RotateKey rotates an active keypair with a specified validity duration and returns the updated key.
	RotateKey(ctx context.Context, keyID string, duration time.Duration) (Key, error)
	// RevokeKey immediately revokes an active keypair.
	RevokeKey(ctx context.Context, keyID string) error
	// PublishDid publishes a DID document to the public resolver and returns its state.
	PublishDid(ctx context.Context, didID string) (DidState, error)
	// UnpublishDid removes a published DID document and returns its state.
	UnpublishDid(ctx context.Context, didID string) (DidState, error)
	// GetDidState returns the current publication state of a DID.
	GetDidState(ctx context.Context, didID string) (DidState, error)
	// AddServiceEndpoint binds a service endpoint to a published DID and returns the updated DID.
	AddServiceEndpoint(ctx context.Context, didID string, endpoint ServiceEndpointPlan) (Did, error)
	// RemoveServiceEndpoint removes a service endpoint from a published DID and returns the updated DID.
	RemoveServiceEndpoint(ctx context.Context, didID string, endpointID string) (Did, error)
	// StoreCredential imports an already signed verifiable credential and returns it.
	StoreCredential(ctx context.Context, cred *CredentialImportPlan) (Credential, error)
	// GetCredentialsByType queries stored credentials matching a type string.
	GetCredentialsByType(ctx context.Context, vcType string) ([]Credential, error)
	// RequestDcpCredential initiates an asynchronous DCP credential request.
	RequestDcpCredential(ctx context.Context, req *DcpCredentialRequestPlan) (string, error)
	// GetDcpRequestStatus inspects the status of an ongoing DCP request.
	GetDcpRequestStatus(ctx context.Context, requestID string) (DcpRequestStatus, error)
	// CreateParticipant creates an EDC participant context and returns it.
	CreateParticipant(ctx context.Context, plan *ParticipantPlan) (Participant, error)
	// GetParticipant resolves an EDC participant context by its ID.
	GetParticipant(ctx context.Context, participantID string) (Participant, error)
	// SetParticipantState activates or deactivates an EDC participant context and returns it.
	SetParticipantState(ctx context.Context, participantID string, active bool) (Participant, error)
	// RegenerateParticipantToken rotates the API token for a participant context.
	RegenerateParticipantToken(ctx context.Context, participantID string) (string, error)
	// UpdateParticipantToken updates the in-memory API token for a participant context.
	UpdateParticipantToken(ctx context.Context, participantID string, token string) error
}

// ===== Pem Descriptor ===========================================

// PemInspector is the port that reads key material. It exists so the domain can
// state what a registrable key is without importing a JOSE library to find out.
type PemInspector interface {
	// Inspect derives a descriptor from PEM-encoded material, reporting an
	// error whose message is safe to hand back to the caller.
	Inspect(pem string) (PemDescriptor, error)
}

// Clock is injected so credential expiry and DID timestamps stay deterministic
// under test.
type Clock interface {
	Now() time.Time
}
