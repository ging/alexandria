// Package fafnir provides the driven HTTP adapter communicating with the remote Fafnir wallet.
// It satisfies the wallet.Wallet domain port across identity and VC operations.
package fafnir

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/caparicio-esd/alexandria/internal/ssi-auth/wallet"
	"resty.dev/v3"
)

const defaultTimeout = 10 * time.Second

var _ wallet.Wallet = (*Adapter)(nil)

// Adapter talks to a Fafnir wallet over its HTTP API.
type Adapter struct {
	http   *resty.Client
	logger *slog.Logger
}

// New builds an adapter against the Fafnir instance at baseURL, which must be
// absolute: a relative one would send every call to whatever host the process
// happens to resolve, and fail late rather than here. A nil logger falls back to
// the default one.
func New(baseURL string, logger *slog.Logger) (*Adapter, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("fafnir: parsing base url %q: %w", baseURL, err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("fafnir: base url %q must be absolute", baseURL)
	}

	client := resty.New().
		SetBaseURL(strings.TrimSuffix(baseURL, "/")).
		SetTimeout(defaultTimeout).
		SetHeader("Accept", "application/json")

	if logger == nil {
		logger = slog.Default()
	}

	return &Adapter{http: client, logger: logger}, nil
}

// Close releases the transport held by the underlying client
func (a *Adapter) Close() error {
	return a.http.Close()
}

// Link refreshes the wallet identity from whatever the remote considers its
// default DID. It satisfies the wallet.Wallet port.
func (a *Adapter) Link(ctx context.Context) (wallet.Did, error) {
	const path = "/dids/default"

	var out didResp

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodGet, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodGet, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}
	if out.Did == "" {
		return wallet.Did{}, fmt.Errorf("fafnir: %s returned an empty did: %w", path, common.ErrNotFound)
	}

	// send back to domain
	return out.ToDomain()
}

// RegisterKey imports raw PEM key material into the wallet and returns the registered key.
func (a *Adapter) RegisterKey(ctx context.Context, keyPlan *wallet.KeyPlan) (wallet.Key, error) {
	const path = "/keys/new"

	// validate input
	if keyPlan == nil {
		return wallet.Key{}, fmt.Errorf("fafnir: %s needs a key plan: %w", path, common.ErrInvalidInput)
	}

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetBody(newKeyReq(*keyPlan)).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Key{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Key{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	// Query keys to find the registered key
	keys, err := a.GetAllKeys(ctx)
	if err == nil {
		for _, k := range keys {
			if k.ID == keyPlan.ID || (keyPlan.Alias != "" && k.Alias == keyPlan.Alias) {
				return k, nil
			}
		}
	}

	return wallet.Key{
		ID:        keyPlan.ID,
		Alias:     keyPlan.Alias,
		Kty:       "RSA",
		CreatedAt: time.Now(),
	}, nil
}

// GetAllKeys lists every key the wallet holds. It satisfies the wallet.Wallet
// port.
func (a *Adapter) GetAllKeys(ctx context.Context) ([]wallet.Key, error) {
	const path = "/keys/all"

	var out []keyResp

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodGet, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return nil, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodGet, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	// send back to domain
	keys := make([]wallet.Key, 0, len(out))
	for _, k := range out {
		d, err := k.ToDomain()
		if err != nil {
			return []wallet.Key{}, err
		}

		keys = append(keys, d)
	}

	return keys, nil
}

// DeleteKey drops a key from the wallet, provided no DID still references it.
func (a *Adapter) DeleteKey(ctx context.Context, keyID string) error {
	path := fmt.Sprintf("/keys/%s", keyID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodDelete, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodDelete, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RegisterDid asks the wallet to mint a DID from the given builder and bind the
// referenced keys into it, returning the minted DID record.
func (a *Adapter) RegisterDid(
	ctx context.Context,
	didPlan *wallet.DidPlan,
) (wallet.Did, error) {
	const path = "/dids/new"

	// validate input
	if didPlan == nil {
		return wallet.Did{}, fmt.Errorf("fafnir: %s needs a did plan: %w", path, common.ErrInvalidInput)
	}
	didReq, err := newDidReq(*didPlan)
	if err != nil {
		return wallet.Did{}, fmt.Errorf("fafnir: %s needs a did correct plan: %w", path, err)
	}

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetBody(&didReq).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	dids, err := a.GetAllDids(ctx)
	if err == nil {
		for _, d := range dids {
			if d.Alias == didPlan.Alias {
				return d, nil
			}
		}
		if len(dids) > 0 {
			return dids[len(dids)-1], nil
		}
	}

	return wallet.Did{Alias: didPlan.Alias}, nil
}

// DeleteDid drops a DID record and its verification method bindings.
func (a *Adapter) DeleteDid(ctx context.Context, didID string) error {
	path := fmt.Sprintf("/dids/%s", didID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodDelete, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodDelete, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// SetDefaultDid promotes a DID to be the wallet active identity and returns it.
func (a *Adapter) SetDefaultDid(ctx context.Context, didID string) (wallet.Did, error) {
	path := fmt.Sprintf("/dids/default/%s", didID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	return a.GetDidByID(ctx, didID)
}

// GetAllDids lists every DID the wallet holds.
func (a *Adapter) GetAllDids(ctx context.Context) ([]wallet.Did, error) {
	const path = "/dids/all"

	var out []didResp

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodGet, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return nil, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()
	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodGet, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	// send back to domain
	dids := make([]wallet.Did, 0, len(out))
	for _, d := range out {
		dom, err := d.ToDomain()
		if err != nil {
			return []wallet.Did{}, err
		}

		dids = append(dids, dom)
	}

	return dids, nil
}

// GetDidByID resolves a DID by its identifier string.
func (a *Adapter) GetDidByID(ctx context.Context, didID string) (wallet.Did, error) {
	path := fmt.Sprintf("/dids/%s", didID)

	var out didResp

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodGet, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodGet, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	return out.ToDomain()
}

// AddKeyToDid binds a key into the verification methods of a DID and returns the updated DID.
func (a *Adapter) AddKeyToDid(ctx context.Context, didID string, keyID string) (wallet.Did, error) {
	path := fmt.Sprintf("/dids/%s/key/%s", didID, keyID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	return a.GetDidByID(ctx, didID)
}

// RemoveKeyFromDid unbinds a key from the verification methods of a DID and returns the updated DID.
func (a *Adapter) RemoveKeyFromDid(ctx context.Context, didID string, keyID string) (wallet.Did, error) {
	path := fmt.Sprintf("/dids/%s/key/%s", didID, keyID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodDelete, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodDelete, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	return a.GetDidByID(ctx, didID)
}

// SetDefaultKey promotes a key to be the default verification method of a DID and returns the updated DID.
func (a *Adapter) SetDefaultKey(ctx context.Context, didID string, keyID string) (wallet.Did, error) {
	path := fmt.Sprintf("/dids/%s/key/default/%s", didID, keyID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return wallet.Did{}, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return wallet.Did{}, statusError(res.StatusCode(), path, res.Bytes())
	}

	return a.GetDidByID(ctx, didID)
}

// WalletInfo returns metadata about the Fafnir wallet instance.
func (a *Adapter) WalletInfo(c context.Context) (wallet.WalletInfo, error) {
	dids, err := a.GetAllDids(c)
	if err != nil {
		return wallet.WalletInfo{}, fmt.Errorf("fafnir: getting all dids: %w", err)
	}
	out := wallet.WalletInfo{
		ID:         "fafnir-local",
		Name:       "fafnir-wallet",
		CreatedAt:  time.Now(),
		AddedAt:    time.Now(),
		Permission: "Administrator",
		Dids:       dids,
	}

	return out, nil
}

// GetAllCredentials lists every Verifiable Credential the wallet holds.
func (a *Adapter) GetAllCredentials(ctx context.Context) ([]wallet.Credential, error) {
	const path = "/vcs/all"

	var out []vcResp

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetResult(&out).
		Get(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodGet, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return nil, fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodGet, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	// send back to domain
	credentials := make([]wallet.Credential, 0, len(out))
	for _, v := range out {
		cred, err := v.ToDomain()
		if err != nil {
			return nil, err
		}

		credentials = append(credentials, cred)
	}

	return credentials, nil
}

// DeleteCredential purges a Verifiable Credential from the wallet storage.
func (a *Adapter) DeleteCredential(ctx context.Context, credentialID string) error {
	path := fmt.Sprintf("/vcs/%s", credentialID)

	// call
	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		Delete(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodDelete, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodDelete, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	// validate
	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// ProcessOid4vci sends an inbound OID4VCI credential offer URI to the wallet.
func (a *Adapter) ProcessOid4vci(ctx context.Context, uri string) error {
	const path = "/oid4vci"

	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("fafnir: %s needs a uri: %w", path, common.ErrInvalidInput)
	}

	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetBody(oidcURIReq{URI: uri}).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// ProcessOid4vp sends an outbound OID4VP presentation request URI to the wallet.
func (a *Adapter) ProcessOid4vp(ctx context.Context, uri string) error {
	const path = "/oid4vp"

	if strings.TrimSpace(uri) == "" {
		return fmt.Errorf("fafnir: %s needs a uri: %w", path, common.ErrInvalidInput)
	}

	started := time.Now()
	res, err := a.http.R().
		SetContext(ctx).
		SetBody(oidcURIReq{URI: uri}).
		Post(path)
	if err != nil {
		a.logger.DebugContext(ctx, "wallet call failed",
			"method", http.MethodPost, "path", path,
			"duration_ms", time.Since(started).Milliseconds(), "err", err)

		return fmt.Errorf("fafnir: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	a.logger.DebugContext(ctx, "wallet call",
		"method", http.MethodPost, "path", path,
		"status", res.StatusCode(), "duration_ms", time.Since(started).Milliseconds())

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// ===== HELPERS ===============================================================

// statusError turns a non-2xx response into a domain error, so callers can use
// errors.Is against the wallet sentinels without knowing HTTP exists.
func statusError(status int, path string, body []byte) error {
	var sentinel error

	switch status {
	case http.StatusNotFound:
		sentinel = common.ErrNotFound
	case http.StatusConflict:
		sentinel = common.ErrConflict
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		sentinel = common.ErrInvalidInput
	default:
		sentinel = errors.New("unexpected status")
	}

	return common.NewUpstreamError("Fafnir", status, path, body, sentinel)
}
