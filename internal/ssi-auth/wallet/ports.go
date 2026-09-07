// ports defines the outbound driven interfaces required by the wallet domain.
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
