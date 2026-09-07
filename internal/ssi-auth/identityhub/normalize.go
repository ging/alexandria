// Package identityhub provides normalization functions between IdentityHub DTOs and domain models.
// It translates wire representations of participants, keys, and credentials into wallet domain types.
package identityhub

import (
	"encoding/json"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"github.com/trustbloc/did-go/doc/did"
)

// normalizeKey converts a KeyPairDto to a domain wallet.Key.
func normalizeKey(dto KeyPairDto) wallet.Key {
	kty := "OKP"
	if dto.Descriptor != nil && dto.Descriptor.Type != "" {
		kty = dto.Descriptor.Type
	}

	return wallet.Key{
		ID:        dto.KeyID,
		Alias:     dto.KeyID,
		Kty:       kty,
		State:     string(dto.State),
		CreatedAt: dto.Timestamp.Time(),
	}
}

// normalizeKeys converts a slice of KeyPairDto to domain wallet.Keys.
func normalizeKeys(dtos []KeyPairDto) []wallet.Key {
	keys := make([]wallet.Key, 0, len(dtos))
	for _, k := range dtos {
		keys = append(keys, normalizeKey(k))
	}
	return keys
}

// normalizeCredential converts a VerifiableCredentialResourceDto to a domain wallet.Credential.
func normalizeCredential(dto VerifiableCredentialResourceDto) wallet.Credential {
	raw := json.RawMessage(dto.VerifiableCredential.RawVc)
	return wallet.Credential{
		ID:             dto.ID,
		VcBody:         raw,
		VcFormat:       dto.VerifiableCredential.Format,
		HolderDid:      dto.HolderID,
		IssuerDid:      dto.IssuerID,
		ParsedDocument: raw,
	}
}

// normalizeCredentials converts a slice of VerifiableCredentialResourceDto to domain wallet.Credentials.
func normalizeCredentials(dtos []VerifiableCredentialResourceDto) []wallet.Credential {
	creds := make([]wallet.Credential, 0, len(dtos))
	for _, c := range dtos {
		creds = append(creds, normalizeCredential(c))
	}
	return creds
}

// normalizeParticipant converts a ParticipantContextDto to a domain wallet.Participant.
func normalizeParticipant(dto ParticipantContextDto) wallet.Participant {
	return wallet.Participant{
		ID:        dto.ParticipantContextID,
		Did:       dto.Did,
		Active:    dto.State == "ACTIVATED" || dto.State == "ACTIVE" || dto.State == "200",
		Roles:     dto.Roles,
		CreatedAt: dto.CreatedAt.Time(),
	}
}

// normalizeDid converts participant information to a domain wallet.Did.
func normalizeDid(pid string, didStr string) wallet.Did {
	doc := did.Doc{
		ID: didStr,
	}

	return wallet.Did{
		ID:       didStr,
		Method:   common.MethodWeb,
		Alias:    pid,
		Default:  true,
		Document: doc,
	}
}
