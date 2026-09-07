// Package identityhub provides DTOs for the Eclipse EDC IdentityHub REST API.
// It models JSON wire structures for participant contexts, keypairs, DIDs, and VCs.
package identityhub

import (
	"encoding/json"
	"fmt"
	"time"
)

// EpochMillis handles parsing timestamps represented either as numeric milliseconds or RFC3339 strings.
type EpochMillis int64

// Time converts the epoch milliseconds to time.Time.
func (e EpochMillis) Time() time.Time {
	if e == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(e))
}

// UnmarshalJSON unmarshals either an int64 epoch timestamp or an RFC3339 string.
func (e *EpochMillis) UnmarshalJSON(b []byte) error {
	var n int64
	if err := json.Unmarshal(b, &n); err == nil {
		*e = EpochMillis(n)
		return nil
	}

	var s string
	if err := json.Unmarshal(b, &s); err == nil && s != "" {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			*e = EpochMillis(t.UnixMilli())
			return nil
		}
	}
	return nil
}

// StateString handles unmarshaling states represented either as integer codes or string names.
type StateString string

// UnmarshalJSON unmarshals either an integer enum code or a string name.
func (s *StateString) UnmarshalJSON(b []byte) error {
	var code int
	if err := json.Unmarshal(b, &code); err == nil {
		switch code {
		case 100:
			*s = "CREATED"
		case 200:
			*s = "ACTIVATED"
		case 300:
			*s = "DEACTIVATED"
		case 400:
			*s = "REVOKED"
		default:
			*s = StateString(fmt.Sprintf("%d", code))
		}
		return nil
	}

	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = StateString(str)
		return nil
	}
	return nil
}

func (s StateString) String() string {
	return string(s)
}

// DidStateString handles unmarshaling DID publication states.
type DidStateString string

// UnmarshalJSON unmarshals either an integer enum code or a string name for DID state.
func (s *DidStateString) UnmarshalJSON(b []byte) error {
	var code int
	if err := json.Unmarshal(b, &code); err == nil {
		switch code {
		case 100:
			*s = "INITIAL"
		case 200:
			*s = "GENERATED"
		case 300:
			*s = "PUBLISHED"
		case 400:
			*s = "UNPUBLISHED"
		default:
			*s = DidStateString(fmt.Sprintf("%d", code))
		}
		return nil
	}

	var str string
	if err := json.Unmarshal(b, &str); err == nil {
		*s = DidStateString(str)
		return nil
	}
	return nil
}

func (s DidStateString) String() string {
	return string(s)
}

// ParticipantContextDto is the wire representation of an IdentityHub participant context.
type ParticipantContextDto struct {
	ParticipantContextID string      `json:"participantContextId"`
	Did                  string      `json:"did"`
	State                StateString `json:"state"`
	APITokenAlias        string      `json:"apiTokenAlias,omitempty"`
	Roles                []string    `json:"roles,omitempty"`
	CreatedAt            EpochMillis `json:"createdAt"`
}

// CreateParticipantDto is the payload for provisioning a participant context.
type CreateParticipantDto struct {
	ParticipantContextID string            `json:"participantContextId"`
	Did                  string            `json:"did"`
	Active               bool              `json:"active"`
	Roles                []string          `json:"roles,omitempty"`
	Key                  *KeyDescriptorDto `json:"key,omitempty"`
}

// KeyDescriptorDto describes asymmetric key material in IdentityHub.
type KeyDescriptorDto struct {
	KeyID              string         `json:"keyId"`
	Type               string         `json:"type,omitempty"`
	PrivateKeyAlias    string         `json:"privateKeyAlias,omitempty"`
	KeyGeneratorParams map[string]any `json:"keyGeneratorParams,omitempty"`
	PublicKeyJwk       map[string]any `json:"publicKeyJwk,omitempty"`
	PublicKeyPem       string         `json:"publicKeyPem,omitempty"`
	Active             bool           `json:"active,omitempty"`
	Usage              []string       `json:"usage,omitempty"`
	KeyContext         *string        `json:"keyContext,omitempty"`
	Properties         map[string]any `json:"properties,omitempty"`
	ResourceURL        *string        `json:"resourceUrl,omitempty"`
}

// KeyPairDto represents an asymmetric key pair bound to a participant context.
type KeyPairDto struct {
	ID                   string            `json:"id"`
	KeyID                string            `json:"keyId"`
	ParticipantContextID string            `json:"participantContextId"`
	State                StateString       `json:"state"`
	Timestamp            EpochMillis       `json:"timestamp"`
	KeyContext           *string           `json:"keyContext,omitempty"`
	Descriptor           *KeyDescriptorDto `json:"descriptor,omitempty"`
}

// DidDocumentPublishDto contains DID document details for publication.
type DidDocumentPublishDto struct {
	Did            string         `json:"did"`
	RawDidDocument map[string]any `json:"rawDidDocument,omitempty"`
}

// DidStateDto reports the resolver publication state for a DID.
type DidStateDto struct {
	Did   string         `json:"did"`
	State DidStateString `json:"state"`
}

// ServiceEndpointDto is the wire representation for a DID service endpoint.
type ServiceEndpointDto struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	ServiceEndpoint string `json:"serviceEndpoint"`
}

// VerifiableCredentialResourceDto represents a stored credential entry in IdentityHub.
type VerifiableCredentialResourceDto struct {
	ID                   string                   `json:"id"`
	ParticipantContextID string                   `json:"participantContextId"`
	HolderID             string                   `json:"holderId"`
	IssuerID             string                   `json:"issuerId"`
	VerifiableCredential VerifiableCredentialBody `json:"verifiableCredential"`
}

// VerifiableCredentialBody contains format and raw content of a stored credential.
type VerifiableCredentialBody struct {
	Format string `json:"format"`
	RawVc  string `json:"rawVc"`
}

// StoreCredentialDto is the wire payload to store a verifiable credential manually.
type StoreCredentialDto struct {
	ID    string `json:"id"`
	RawVc string `json:"rawVc"`
}

// DcpCredentialRequestDto is the wire payload to request credentials via DCP.
type DcpCredentialRequestDto struct {
	IssuerURL string   `json:"issuerUrl"`
	HolderPid string   `json:"holderPid"`
	Types     []string `json:"types"`
	Format    string   `json:"format"`
}

// DcpRequestStatusDto reports the status of an asynchronous DCP request.
type DcpRequestStatusDto struct {
	RequestID string `json:"requestId"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
}

// TokenResponseDto represents a regenerated API token response.
type TokenResponseDto struct {
	Token string `json:"token"`
}
