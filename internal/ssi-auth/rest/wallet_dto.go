// wallet_dto defines the public REST transfer objects for wallet operations.
// It maps domain identity and credential models to external JSON wire structures.
package rest

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"github.com/trustbloc/did-go/doc/did"
)

// ===== DID rest DTOs =========================================================

type didIDResp struct {
	Did string `json:"did"`
}

type keyBindingResp struct {
	Fragment string `json:"fragment"`
}

// didResp is the public representation of a wallet DID.
type didResp struct {
	ID                 string           `json:"id"`
	Method             string           `json:"method"`
	Alias              string           `json:"alias,omitempty"`
	Default            bool             `json:"default"`
	Keys               []keyBindingResp `json:"keys"`
	DefaultKeyFragment string           `json:"defaultKeyFragment"`
	Document           *did.Doc         `json:"didDocument"`
}

// newDidResp projects a domain DID onto the wire.
func newDidResp(d wallet.Did) didResp {
	keys := make([]keyBindingResp, 0, len(d.Keys))
	for _, k := range d.Keys {
		keys = append(keys, keyBindingResp{Fragment: k.Fragment})
	}

	return didResp{
		ID:                 d.ID,
		Method:             string(d.Method),
		Alias:              d.Alias,
		Default:            d.Default,
		Keys:               keys,
		DefaultKeyFragment: d.DefaultKey.Fragment,
		Document:           &d.Document,
	}
}

// newDidResp projects a domain DID onto the wire.
func newDidResps(dids []wallet.Did) []didResp {
	out := make([]didResp, 0, len(dids))
	for _, d := range dids {
		out = append(out, newDidResp(d))
	}

	return out
}

// ===== KEY rest DTOs =========================================================

// keyResp is the public representation of a wallet key.
type keyResp struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias,omitempty"`
	Kty       string    `json:"kty"`
	Crv       *string   `json:"crv,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// newKeyResp projects a domain key onto the wire.
func newKeyResp(k wallet.Key) keyResp {
	return keyResp{
		ID:        k.ID,
		Alias:     k.Alias,
		Kty:       k.Kty,
		Crv:       k.Crv,
		CreatedAt: k.CreatedAt,
	}
}

// newKeyResps projects a list of domain keys, never nil: an empty wallet is an
// empty JSON array, not null.
func newKeyResps(keys []wallet.Key) []keyResp {
	out := make([]keyResp, 0, len(keys))
	for _, k := range keys {
		out = append(out, newKeyResp(k))
	}

	return out
}

type registerKeyReq struct {
	ID    *string `json:"id,omitempty"`
	Pem   string  `json:"pem"`
	Alias string  `json:"alias,omitempty"`
}

// ===== DID Registering rest DTOs =========================================================

type registerDidReq struct {
	Builder didBuilderReq   `json:"builder"`
	Keys    []string        `json:"keys"`
	Alias   string          `json:"alias,omitempty"`
	Service []didServiceReq `json:"service,omitempty"`
}

type didServiceReq struct {
	ID              string                `json:"id,omitempty"`
	Type            common.DidServiceType `json:"type"`
	ServiceEndpoint string                `json:"serviceEndpoint"`
}

func servicesToDomain(reqs []didServiceReq) ([]common.DidService, error) {
	services := make([]common.DidService, 0, len(reqs))

	for i, r := range reqs {
		service := common.DidService{
			ID:       r.ID,
			Type:     r.Type,
			Endpoint: r.ServiceEndpoint,
		}

		if err := service.Validate(); err != nil {
			return nil, fmt.Errorf("service %d: %w", i, err)
		}

		services = append(services, service)
	}

	return services, nil
}

type didBuilderReq struct {
	builder common.DidBuilder
}

type jwkBuilderReq struct {
	Pem string `json:"pem"`
}

type webBuilderReq struct {
	Domain string  `json:"domain"`
	Port   *string `json:"port,omitempty"`
	Path   *string `json:"path,omitempty"`
}

func (b *didBuilderReq) UnmarshalJSON(data []byte) error {
	var peek struct {
		Method string `json:"method"`
	}

	if err := json.Unmarshal(data, &peek); err != nil {
		return fmt.Errorf("reading did builder: %w", err)
	}

	method, err := common.ParseMethod(peek.Method)
	if err != nil {
		return err
	}

	switch method {
	case common.MethodJwk:
		var v jwkBuilderReq
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("reading did:jwk builder: %w", err)
		}

		b.builder = common.JwkDidBuilder{Pem: v.Pem}
	case common.MethodWeb:
		var v webBuilderReq
		if err := json.Unmarshal(data, &v); err != nil {
			return fmt.Errorf("reading did:web builder: %w", err)
		}

		b.builder = common.WebDidBuilder{Domain: v.Domain, Port: v.Port, Path: v.Path}
	}

	return nil
}

func (b didBuilderReq) toDomain() (common.DidBuilder, error) {
	if b.builder == nil {
		return nil, fmt.Errorf("%w: no did builder given", common.ErrInvalidInput)
	}

	if err := b.builder.Validate(); err != nil {
		return nil, err
	}

	return b.builder, nil
}

// ===== Wallet Info DTO =========================================================

type walletInfoRest struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	CreatedAt  time.Time `json:"createdAt"`
	AddedAt    time.Time `json:"addedAt"`
	Permission string    `json:"permission"`
	Dids       []didResp `json:"dids"`
}

// newKeyResp projects a domain key onto the wire.
func newWalletInfoRest(i wallet.WalletInfo) walletInfoRest {
	return walletInfoRest{
		ID:         i.ID,
		Name:       i.Name,
		CreatedAt:  i.CreatedAt,
		AddedAt:    i.AddedAt,
		Permission: i.Permission,
		Dids:       newDidResps(i.Dids),
	}
}

// ===== Credential rest DTOs ==================================================

// credentialResp is the public representation of a stored Verifiable Credential.
type credentialResp struct {
	ID             string          `json:"id"`
	VcBody         json.RawMessage `json:"vcBody"`
	VcType         string          `json:"vcType"`
	VcFormat       string          `json:"vcFormat"`
	HolderDid      string          `json:"holderDid"`
	IssuerDid      string          `json:"issuerDid"`
	ParsedDocument json.RawMessage `json:"parsedDocument"`
	ValidUntil     *time.Time      `json:"validUntil,omitempty"`
	AddedOn        time.Time       `json:"addedOn"`
}

// newCredentialResp projects a domain credential onto the wire.
func newCredentialResp(c wallet.Credential) credentialResp {
	return credentialResp{
		ID:             c.ID,
		VcBody:         c.VcBody,
		VcType:         c.VcType,
		VcFormat:       c.VcFormat,
		HolderDid:      c.HolderDid,
		IssuerDid:      c.IssuerDid,
		ParsedDocument: c.ParsedDocument,
		ValidUntil:     c.ValidUntil,
		AddedOn:        c.AddedOn,
	}
}

// newCredentialResps projects a list of domain credentials, never nil: an empty wallet is an
// empty JSON array, not null.
func newCredentialResps(credentials []wallet.Credential) []credentialResp {
	out := make([]credentialResp, 0, len(credentials))
	for _, c := range credentials {
		out = append(out, newCredentialResp(c))
	}

	return out
}

// ===== Protocol rest DTOs ====================================================

type oidcUriReq struct {
	URI string `json:"uri"`
}
