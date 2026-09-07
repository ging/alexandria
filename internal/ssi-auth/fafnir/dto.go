// dto defines Fafnir wire structures and their conversions to domain models.
// It encapsulates remote API schema representations and anti-corruption parsing.
package fafnir

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/jose"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"github.com/trustbloc/did-go/doc/did"
)

// ===== DID related DTO's =====================================================

type didResp struct {
	// ID is the record identifier inside Fafnir, not the DID itself.
	ID string `json:"id"`
	// Did is the identifier proper, e.g. "did:web:alexandria.upm.es".
	Did string `json:"did"`
	// Alias is the human-readable tag the DID was registered under.
	Alias string `json:"alias"`
	// Default reports whether this is the wallet active identity.
	Default bool `json:"default"`
	// Type is the DID method as Fafnir spells it: "jwk" or "web".
	Type string `json:"type"`
	// Keys lists every key bound into the DID verification methods.
	Keys []keyRef `json:"keys"`
	// DefaultKey is the key used for signing unless told otherwise.
	DefaultKey keyRef `json:"default_key"`
	// DidDocument is held raw on purpose. did.Doc validates against DID Core on
	// unmarshal, so decoding it inline would make one malformed document throw
	// away the whole record — id, alias, keys and all. Call Doc to parse it.
	DidDocument json.RawMessage `json:"did_document"`
	// Service holds the published service entries. Fafnir sends null when there
	// are none, which decodes to a nil slice.
	Service []did.Service `json:"service,omitempty"`
}

// keyRef is how Fafnir points at a stored key: an internal storage path plus the
// fragment the key is published under in the DID Document.
type keyRef struct {
	Internal string `json:"internal"`
	Fragment string `json:"fragment"`
}

func (d didResp) ToDomain() (wallet.Did, error) {
	method, err := common.ParseMethod(d.Type)
	if err != nil {
		return wallet.Did{}, fmt.Errorf("fafnir: did %q: %w", d.Did, err)
	}

	doc, err := d.document()
	if err != nil {
		return wallet.Did{}, fmt.Errorf("fafnir: did %q: %w", d.Did, err)
	}

	keys := make([]wallet.KeyBinding, 0, len(d.Keys))
	for _, k := range d.Keys {
		keys = append(keys, k.toDomain())
	}

	return wallet.Did{
		ID:         d.ID,
		Method:     method,
		Alias:      d.Alias,
		Default:    d.Default,
		Document:   *doc,
		Keys:       keys,
		DefaultKey: d.DefaultKey.toDomain(),
	}, nil
}

// document normalizes and parses the embedded DID Document.
func (d didResp) document() (*did.Doc, error) {
	normalized, err := normalizeDidDocument(d.DidDocument)
	if err != nil {
		return nil, err
	}

	return did.ParseDocument(normalized)
}

func (k keyRef) toDomain() wallet.KeyBinding {
	return wallet.KeyBinding{KeyID: k.Internal, Fragment: k.Fragment}
}

type didReq struct {
	Builder didBuilderReq   `json:"builder"`
	KeysID  []string        `json:"keys,omitempty"`
	Alias   string          `json:"alias"`
	Service []didServiceReq `json:"service,omitempty"`
}

type didBuilderReq struct {
	Jwk *jwkConfigReq `json:"Jwk,omitempty"`
	Web *webConfigReq `json:"Web,omitempty"`
}

type jwkConfigReq struct {
	Pem string `json:"pem"`
}

type webConfigReq struct {
	Domain string  `json:"domain"`
	Port   *string `json:"port,omitempty"`
	Path   *string `json:"path,omitempty"`
}

type didServiceReq struct {
	ID       string `json:"id,omitempty"`
	Type     string `json:"type"`
	Endpoint string `json:"serviceEndpoint"`
}

// newDidReq projects a domain DID registration onto the wire.
func newDidReq(
	didPlan wallet.DidPlan,
) (didReq, error) {
	builderReq, err := newDidBuilderReq(didPlan.Builder)
	if err != nil {
		return didReq{}, err
	}

	// Service is a pointer in the domain, so "no services" arrives as nil and
	// dereferencing it unguarded is a panic on the ordinary case of a DID with
	// no endpoints to publish.
	var services []common.DidService
	if didPlan.Service != nil {
		services = *didPlan.Service
	}

	entries := make([]didServiceReq, 0, len(services))
	for _, s := range services {
		entries = append(entries, didServiceReq{
			ID:       s.ID,
			Type:     string(s.Type),
			Endpoint: s.Endpoint,
		})
	}

	return didReq{
		Builder: builderReq,
		KeysID:  didPlan.Keys,
		Alias:   didPlan.Alias,
		Service: entries,
	}, nil
}

// newDidBuilderReq turns the sealed union into its external-tagging form.
func newDidBuilderReq(builder common.DidBuilder) (didBuilderReq, error) {
	switch b := builder.(type) {
	case common.JwkDidBuilder:
		return didBuilderReq{Jwk: &jwkConfigReq{Pem: b.Pem}, Web: nil}, nil

	case common.WebDidBuilder:
		return didBuilderReq{
			Jwk: nil,
			Web: &webConfigReq{Domain: b.Domain, Port: b.Port, Path: b.Path},
		}, nil

	default:
		return didBuilderReq{}, fmt.Errorf("fafnir: did builder %T: %w", builder, common.ErrUnsupported)
	}
}

// ===== KEYS related DTO's =====================================================

type keyReq struct {
	ID    string `json:"id"`
	Alias string `json:"alias"`
	Pem   string `json:"pem"`
}

func newKeyReq(plan wallet.KeyPlan) keyReq {
	return keyReq{ID: plan.ID, Alias: plan.Alias, Pem: plan.Pem}
}

type keyResp struct {
	ID        string    `json:"id"`
	Alias     string    `json:"alias"`
	Kty       string    `json:"kty"`
	Crv       *string   `json:"crv"`
	CreatedAt time.Time `json:"created_at"`
}

func (k keyResp) ToDomain() (wallet.Key, error) {
	if k.ID == "" {
		return wallet.Key{}, fmt.Errorf("fafnir: key record carries no id: %w", common.ErrNotFound)
	}

	if _, err := jose.ParseKty(k.Kty); err != nil {
		return wallet.Key{}, fmt.Errorf("fafnir: key %q: %w", k.ID, err)
	}

	// RSA and symmetric keys carry no curve, so only a present one is checked.
	if k.Crv != nil && *k.Crv != "" {
		if _, err := jose.ParseCrv(*k.Crv); err != nil {
			return wallet.Key{}, fmt.Errorf("fafnir: key %q: %w", k.ID, err)
		}
	}

	return wallet.Key{
		ID:        k.ID,
		Alias:     k.Alias,
		Kty:       k.Kty,
		Crv:       k.Crv,
		CreatedAt: k.CreatedAt,
	}, nil
}

// ===== WalletInfo related DTO's =====================================================

type walletInfoRes struct {
	ID         string
	Name       string
	CreatedAt  string
	AddedAt    string
	Permission string
	Dids       []didResp
}

func (w *walletInfoRes) ToDomain() (wallet.WalletInfo, error) {
	var dids []wallet.Did
	var out wallet.WalletInfo

	entries := make([]didResp, 0, len(w.Dids))
	for _, d := range entries {
		innerDid, err := d.ToDomain()
		if err != nil {
			return wallet.WalletInfo{}, fmt.Errorf("fafnir: couldn't convert did: %w", err)
		}
		dids = append(dids, innerDid)
	}

	layout := "2014-09-12T11:45:26.371Z"
	createdAt, err := time.Parse(layout, w.CreatedAt)
	if err != nil {
		return wallet.WalletInfo{}, fmt.Errorf("fafnir: couldn't convert created at: %w", err)
	}
	addedAt, err := time.Parse(layout, w.AddedAt)
	if err != nil {
		return wallet.WalletInfo{}, fmt.Errorf("fafnir: couldn't convert created at: %w", err)
	}

	out = wallet.WalletInfo{
		ID:         w.ID,
		Name:       w.Name,
		CreatedAt:  createdAt,
		AddedAt:    addedAt,
		Permission: w.Permission,
		Dids:       dids,
	}

	return out, nil
}

// ===== Credential related DTO's ==============================================

type vcResp struct {
	ID             string          `json:"id"`
	VcBody         json.RawMessage `json:"vc_body"`
	VcType         string          `json:"vc_type"`
	VcFormat       string          `json:"vc_format"`
	HolderDid      string          `json:"holder_did"`
	IssuerDid      string          `json:"issuer_did"`
	ParsedDocument json.RawMessage `json:"parsed_document"`
	ValidUntil     *time.Time      `json:"valid_until"`
	AddedOn        time.Time       `json:"added_on"`
}

func (v vcResp) ToDomain() (wallet.Credential, error) {
	if v.ID == "" {
		return wallet.Credential{}, fmt.Errorf("fafnir: credential record carries no id: %w", common.ErrNotFound)
	}

	return wallet.Credential{
		ID:             v.ID,
		VcBody:         v.VcBody,
		VcType:         v.VcType,
		VcFormat:       v.VcFormat,
		HolderDid:      v.HolderDid,
		IssuerDid:      v.IssuerDid,
		ParsedDocument: v.ParsedDocument,
		ValidUntil:     v.ValidUntil,
		AddedOn:        v.AddedOn,
	}, nil
}

// ===== Protocol related DTO's ================================================

type oidcUriReq struct {
	URI string `json:"uri"`
}
