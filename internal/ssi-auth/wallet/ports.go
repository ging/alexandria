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
	// RegisterKey files raw key material under the path the plan names.
	RegisterKey(ctx context.Context, keyPlan *KeyPlan) error
	// Get all keys
	GetAllKeys(ctx context.Context) ([]Key, error)
	// Delete key
	DeleteKey(ctx context.Context, keyID string) error
	// Registers did
	RegisterDid(ctx context.Context, didPlan *DidPlan) error
	// GetAllDids
	GetAllDids(ctx context.Context) ([]Did, error)
	// GetAllDids
	GetDidByID(ctx context.Context, didID string) (Did, error)
	// Delete did
	DeleteDid(ctx context.Context, didID string) error
	// SetDefaultDid
	SetDefaultDid(ctx context.Context, didID string) error
	// AddKeyToDid
	AddKeyToDid(ctx context.Context, didID string, keyID string) error
	// RemoveKeyFromDid
	RemoveKeyFromDid(ctx context.Context, didID string, keyID string) error
	// SetDefaultKey
	SetDefaultKey(ctx context.Context, didID string, keyID string) error
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
	// RotateKey rotates an active keypair with a specified validity duration.
	RotateKey(ctx context.Context, keyID string, duration time.Duration) error
	// RevokeKey immediately revokes an active keypair.
	RevokeKey(ctx context.Context, keyID string) error
	// PublishDid publishes a DID document to the public resolver.
	PublishDid(ctx context.Context, didID string) error
	// UnpublishDid removes a published DID document.
	UnpublishDid(ctx context.Context, didID string) error
	// GetDidState returns the current publication state of a DID.
	GetDidState(ctx context.Context, didID string) (DidState, error)
	// AddServiceEndpoint binds a service endpoint to a published DID.
	AddServiceEndpoint(ctx context.Context, didID string, endpoint ServiceEndpointPlan) error
	// RemoveServiceEndpoint removes a service endpoint from a published DID.
	RemoveServiceEndpoint(ctx context.Context, didID string, endpointID string) error
	// StoreCredential imports an already signed verifiable credential.
	StoreCredential(ctx context.Context, cred *CredentialImportPlan) error
	// GetCredentialsByType queries stored credentials matching a type string.
	GetCredentialsByType(ctx context.Context, vcType string) ([]Credential, error)
	// RequestDcpCredential initiates an asynchronous DCP credential request.
	RequestDcpCredential(ctx context.Context, req *DcpCredentialRequestPlan) (string, error)
	// GetDcpRequestStatus inspects the status of an ongoing DCP request.
	GetDcpRequestStatus(ctx context.Context, requestID string) (DcpRequestStatus, error)
	// CreateParticipant creates an EDC participant context.
	CreateParticipant(ctx context.Context, plan *ParticipantPlan) error
	// GetParticipant resolves an EDC participant context by its ID.
	GetParticipant(ctx context.Context, participantID string) (Participant, error)
	// SetParticipantState activates or deactivates an EDC participant context.
	SetParticipantState(ctx context.Context, participantID string, active bool) error
	// RegenerateParticipantToken rotates the API token for a participant context.
	RegenerateParticipantToken(ctx context.Context, participantID string) (string, error)
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
