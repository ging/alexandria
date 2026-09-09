// Package identityhub provides normalization functions between IdentityHub DTOs and domain models.
// It translates wire representations of participants, keys, and credentials into wallet domain types.
package identityhub

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"strings"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/jose"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"github.com/trustbloc/did-go/doc/did"
)

// determineKeyTypeAndCurve inspects the serialized public key or descriptor to derive kty and crv.
func determineKeyTypeAndCurve(dto KeyPairDto) (string, *string) {
	if dto.SerializedPublicKey != "" {
		s := strings.TrimSpace(dto.SerializedPublicKey)

		// 1. Check if it's a JSON JWK
		if strings.HasPrefix(s, "{") {
			var jwkStruct struct {
				Kty string `json:"kty"`
				Crv string `json:"crv"`
			}
			if err := json.Unmarshal([]byte(s), &jwkStruct); err == nil && jwkStruct.Kty != "" {
				var crv *string
				if jwkStruct.Crv != "" {
					crv = &jwkStruct.Crv
				}
				return jwkStruct.Kty, crv
			}
		}

		// 2. Check if it's a PEM string
		if strings.HasPrefix(s, "-----BEGIN") {
			block, _ := pem.Decode([]byte(s))
			if block != nil {
				if pub, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
					switch p := pub.(type) {
					case *rsa.PublicKey:
						return "RSA", nil
					case *ecdsa.PublicKey:
						c := p.Curve.Params().Name
						return "EC", &c
					case ed25519.PublicKey:
						c := "Ed25519"
						return "OKP", &c
					}
				}
				if _, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil {
					return "RSA", nil
				}
			}

			// Fallback to jose.ParsePEM for extended formats/curves
			if key, err := jose.ParsePEM([]byte(s)); err == nil {
				kty := key.KeyType().String()
				var crv *string
				var crvStr string
				if err := key.Get("crv", &crvStr); err == nil && crvStr != "" {
					crv = &crvStr
				} else if crvAlg, ok := jose.CurveOf(key); ok {
					c := crvAlg.String()
					crv = &c
				}
				return kty, crv
			}
		}
	}

	// 3. Fall back to descriptor if available
	if dto.Descriptor != nil && dto.Descriptor.Type != "" {
		switch strings.ToUpper(dto.Descriptor.Type) {
		case "EDDSA", "ED25519":
			c := "Ed25519"
			return "OKP", &c
		case "ECDSA", "ES256":
			c := "P-256"
			return "EC", &c
		case "RSA":
			return "RSA", nil
		default:
			return dto.Descriptor.Type, nil
		}
	}

	return "OKP", nil
}

// normalizeKey converts a KeyPairDto to a domain wallet.Key.
func normalizeKey(dto KeyPairDto) wallet.Key {
	kty, crv := determineKeyTypeAndCurve(dto)

	alias := dto.KeyID
	if dto.PrivateKeyAlias != "" {
		alias = dto.PrivateKeyAlias
	}

	var serializedPubKey *string
	if dto.SerializedPublicKey != "" {
		s := dto.SerializedPublicKey
		serializedPubKey = &s
	}

	keyContext := dto.KeyContext
	if keyContext == nil && dto.Descriptor != nil && dto.Descriptor.KeyContext != nil {
		keyContext = dto.Descriptor.KeyContext
	}

	usage := dto.Usage
	if len(usage) == 0 && dto.Descriptor != nil && len(dto.Descriptor.Usage) > 0 {
		usage = dto.Descriptor.Usage
	}

	var defaultPairPtr *bool
	if dto.DefaultPair {
		dp := true
		defaultPairPtr = &dp
	} else if dto.Descriptor != nil && dto.Descriptor.Active {
		act := dto.Descriptor.Active
		defaultPairPtr = &act
	}

	var privKeyAliasPtr *string
	if dto.PrivateKeyAlias != "" {
		p := dto.PrivateKeyAlias
		privKeyAliasPtr = &p
	}

	createdAt := dto.Timestamp.Time()
	if dto.CreatedAt != 0 {
		createdAt = dto.CreatedAt.Time()
	}

	return wallet.Key{
		ID:                  dto.KeyID,
		Alias:               alias,
		Kty:                 kty,
		Crv:                 crv,
		State:               string(dto.State),
		CreatedAt:           createdAt,
		SerializedPublicKey: serializedPubKey,
		KeyContext:          keyContext,
		Usage:               usage,
		DefaultPair:         defaultPairPtr,
		PrivateKeyAlias:     privKeyAliasPtr,
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

// parseVcToken inspects rawVc whether it is a JWT/JOSE token or raw JSON-LD and extracts structured components.
func parseVcToken(rawVc string) (credJSON json.RawMessage, types []string, issuanceDate *time.Time, validUntil *time.Time) {
	s := strings.TrimSpace(rawVc)
	if s == "" {
		return nil, nil, nil, nil
	}

	// 1. If it's a JWT/JWS (header.payload.signature)
	if strings.Count(s, ".") == 2 {
		parts := strings.Split(s, ".")
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			payloadBytes, err = base64.URLEncoding.DecodeString(parts[1])
		}
		if err == nil {
			var payload map[string]any
			if err := json.Unmarshal(payloadBytes, &payload); err == nil {
				// Check W3C VC 1.0 "vc" wrapper
				if vcMap, ok := payload["vc"].(map[string]any); ok {
					if b, err := json.Marshal(vcMap); err == nil {
						credJSON = json.RawMessage(b)
					}
					types = extractTypes(vcMap["type"])
					issuanceDate = extractDate(payload["nbf"], vcMap["issuanceDate"])
					validUntil = extractDate(payload["exp"], vcMap["expirationDate"])
					return credJSON, types, issuanceDate, validUntil
				}

				// Check W3C VC 2.0 (payload is the credential itself)
				credJSON = json.RawMessage(payloadBytes)
				types = extractTypes(payload["type"])
				issuanceDate = extractDate(payload["nbf"], payload["validFrom"], payload["issuanceDate"])
				validUntil = extractDate(payload["exp"], payload["validUntil"], payload["expirationDate"])
				return credJSON, types, issuanceDate, validUntil
			}
		}
	}

	// 2. If it's plain JSON
	if strings.HasPrefix(s, "{") {
		credJSON = json.RawMessage(s)
		var m map[string]any
		if err := json.Unmarshal([]byte(s), &m); err == nil {
			types = extractTypes(m["type"])
			issuanceDate = extractDate(m["issuanceDate"], m["validFrom"])
			validUntil = extractDate(m["expirationDate"], m["validUntil"])
		}
		return credJSON, types, issuanceDate, validUntil
	}

	return nil, nil, nil, nil
}

func extractTypes(v any) []string {
	var res []string
	switch val := v.(type) {
	case string:
		res = append(res, val)
	case []any:
		for _, item := range val {
			if s, ok := item.(string); ok {
				res = append(res, s)
			}
		}
	}
	return res
}

func extractDate(candidates ...any) *time.Time {
	for _, c := range candidates {
		if c == nil {
			continue
		}
		switch val := c.(type) {
		case float64:
			t := time.Unix(int64(val), 0)
			return &t
		case int64:
			t := time.Unix(val, 0)
			return &t
		case string:
			if t, err := time.Parse(time.RFC3339, val); err == nil {
				return &t
			}
		}
	}
	return nil
}

// normalizeCredential converts a VerifiableCredentialResourceDto to a domain wallet.Credential.
func normalizeCredential(dto VerifiableCredentialResourceDto) wallet.Credential {
	raw := json.RawMessage(dto.VerifiableCredential.RawVc)
	rawVcStr := dto.VerifiableCredential.RawVc
	format := dto.VerifiableCredential.Format

	credObj, types, issuanceDate, validUntil := parseVcToken(rawVcStr)
	if len(credObj) == 0 {
		credObj = raw
	}

	vcType := "VerifiableCredential"
	for _, t := range types {
		if t != "VerifiableCredential" {
			vcType = t
			break
		}
	}

	var pid *string
	if dto.ParticipantContextID != "" {
		p := dto.ParticipantContextID
		pid = &p
	}

	return wallet.Credential{
		ID:                   dto.ID,
		RawVc:                rawVcStr,
		Format:               format,
		Credential:           credObj,
		VcBody:               raw,
		VcType:               vcType,
		VcFormat:             format,
		Types:                types,
		HolderDid:            dto.HolderID,
		IssuerDid:            dto.IssuerID,
		ParsedDocument:       credObj,
		ParticipantContextID: pid,
		ValidUntil:           validUntil,
		IssuanceDate:         issuanceDate,
		AddedOn:              time.Now(),
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

// normalizeDidDoc converts a raw DID document JSON to a domain wallet.Did with full cryptographic key material.
func normalizeDidDoc(pid string, rawDoc []byte) (wallet.Did, error) {
	doc, err := did.ParseDocument(rawDoc)
	if err != nil {
		return wallet.Did{}, fmt.Errorf("parsing did document: %w", err)
	}

	keys := make([]wallet.KeyBinding, 0, len(doc.VerificationMethod))
	for _, vm := range doc.VerificationMethod {
		fragment := vm.ID
		if idx := strings.Index(fragment, "#"); idx >= 0 {
			fragment = fragment[idx+1:]
		}
		keys = append(keys, wallet.KeyBinding{
			KeyID:    fragment,
			Fragment: fragment,
		})
	}

	var defaultKey wallet.KeyBinding
	if len(keys) > 0 {
		defaultKey = keys[0]
	}

	return wallet.Did{
		ID:         doc.ID,
		Method:     common.MethodWeb,
		Alias:      pid,
		Default:    true,
		Document:   *doc,
		Keys:       keys,
		DefaultKey: defaultKey,
	}, nil
}

// normalizeDid converts participant information to a basic domain wallet.Did fallback.
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
