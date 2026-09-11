// Package identityhub provides an HTTP client for Eclipse EDC IdentityHub runtime APIs.
// It executes HTTP calls against Identity API and handles serialization and status mapping.
package identityhub

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/caparicio-esd/alexandria/internal/common"
	"resty.dev/v3"
)

const defaultTimeout = 10 * time.Second

// EdcClient handles HTTP interactions with IdentityHub endpoints.
type EdcClient struct {
	http   *resty.Client
	apiKey string
	mu     sync.RWMutex
	logger *slog.Logger
}

// NewEdcClient constructs an EdcClient targeting the IdentityHub Identity API base URL.
func NewEdcClient(baseURL string, apiKey string, logger *slog.Logger) (*EdcClient, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("identityhub: parsing base url %q: %w", baseURL, err)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("identityhub: base url %q must be absolute", baseURL)
	}

	if logger == nil {
		logger = slog.Default()
	}

	client := resty.New().
		SetBaseURL(strings.TrimSuffix(baseURL, "/")).
		SetTimeout(defaultTimeout).
		SetHeader("Accept", "application/json").
		SetHeader("Content-Type", "application/json")

	if apiKey != "" {
		client.SetHeader("x-api-key", apiKey)
	}

	return &EdcClient{
		http:   client,
		apiKey: apiKey,
		logger: logger,
	}, nil
}

// SetAPIKey updates the API key header used for future HTTP requests.
func (c *EdcClient) SetAPIKey(apiKey string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.apiKey = apiKey
	c.http.SetHeader("x-api-key", apiKey)
}

// APIKey returns the currently active API key.
func (c *EdcClient) APIKey() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.apiKey
}

// Close terminates any idle network connections.
func (c *EdcClient) Close() error {
	return c.http.Close()
}

// GetParticipant fetches participant metadata.
func (c *EdcClient) GetParticipant(ctx context.Context, pid string) (*ParticipantContextDto, error) {
	path := fmt.Sprintf("/participants/%s", url.PathEscape(pid))
	var out ParticipantContextDto

	res, err := c.http.R().SetContext(ctx).SetResult(&out).Get(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	return &out, nil
}

// CreateParticipant provisions a new participant context.
func (c *EdcClient) CreateParticipant(ctx context.Context, req *CreateParticipantDto) error {
	const path = "/participants"

	res, err := c.http.R().SetContext(ctx).SetBody(req).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// SetParticipantState activates or deactivates a participant context.
func (c *EdcClient) SetParticipantState(ctx context.Context, pid string, active bool) error {
	path := fmt.Sprintf("/participants/%s/state?isActive=%t", url.PathEscape(pid), active)

	res, err := c.http.R().SetContext(ctx).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RegenerateParticipantToken rotates the participant API token.
func (c *EdcClient) RegenerateParticipantToken(ctx context.Context, pid string) (string, error) {
	path := fmt.Sprintf("/participants/%s/token", url.PathEscape(pid))

	res, err := c.http.R().SetContext(ctx).Post(path)
	if err != nil {
		return "", fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return "", statusError(res.StatusCode(), path, res.Bytes())
	}

	body := strings.TrimSpace(res.String())
	var out TokenResponseDto
	if err := json.Unmarshal([]byte(body), &out); err == nil && out.Token != "" {
		return out.Token, nil
	}

	token := strings.Trim(body, `"`)
	return token, nil
}

// ListKeys lists all keypairs for a participant.
func (c *EdcClient) ListKeys(ctx context.Context, pid string) ([]KeyPairDto, error) {
	path := fmt.Sprintf("/participants/%s/keypairs", url.PathEscape(pid))
	var out []KeyPairDto

	res, err := c.http.R().SetContext(ctx).SetResult(&out).Get(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	return out, nil
}

// AddKey registers a key descriptor with the participant.
func (c *EdcClient) AddKey(ctx context.Context, pid string, desc *KeyDescriptorDto) error {
	path := fmt.Sprintf("/participants/%s/keypairs", url.PathEscape(pid))

	res, err := c.http.R().SetContext(ctx).SetBody(desc).Put(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RotateKey triggers a key rotation with an active overlap duration.
func (c *EdcClient) RotateKey(ctx context.Context, pid string, keyID string, duration time.Duration) error {
	path := fmt.Sprintf("/participants/%s/keypairs/%s/rotate",
		url.PathEscape(pid), url.PathEscape(keyID))
	if duration > 0 {
		path = fmt.Sprintf("%s?duration=%d", path, duration.Milliseconds())
	}

	res, err := c.http.R().SetContext(ctx).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RevokeKey immediately revokes a keypair.
func (c *EdcClient) RevokeKey(ctx context.Context, pid string, keyID string) error {
	path := fmt.Sprintf("/participants/%s/keypairs/%s/revoke",
		url.PathEscape(pid), url.PathEscape(keyID))

	res, err := c.http.R().SetContext(ctx).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// PublishDid publishes a DID document to the resolver.
func (c *EdcClient) PublishDid(ctx context.Context, pid string, did string) error {
	path := fmt.Sprintf("/participants/%s/dids/publish", url.PathEscape(pid))
	req := DidDocumentPublishDto{Did: did}

	res, err := c.http.R().SetContext(ctx).SetBody(req).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// UnpublishDid removes a published DID document.
func (c *EdcClient) UnpublishDid(ctx context.Context, pid string, did string) error {
	path := fmt.Sprintf("/participants/%s/dids/unpublish", url.PathEscape(pid))
	req := DidDocumentPublishDto{Did: did}

	res, err := c.http.R().SetContext(ctx).SetBody(req).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// GetDidState queries the publication state for a DID.
func (c *EdcClient) GetDidState(ctx context.Context, pid string, did string) (*DidStateDto, error) {
	path := fmt.Sprintf("/participants/%s/dids/state", url.PathEscape(pid))
	req := DidDocumentPublishDto{Did: did}

	res, err := c.http.R().SetContext(ctx).SetBody(req).Post(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	var out DidStateDto
	if err := json.Unmarshal(res.Bytes(), &out); err == nil && out.State != "" {
		if out.Did == "" {
			out.Did = did
		}
		return &out, nil
	}

	stateStr := strings.Trim(strings.TrimSpace(string(res.Bytes())), "\"")

	return &DidStateDto{
		Did:   did,
		State: DidStateString(stateStr),
	}, nil
}

// QueryDids queries all DID documents for a participant context.
func (c *EdcClient) QueryDids(ctx context.Context, pid string) ([]json.RawMessage, error) {
	path := fmt.Sprintf("/participants/%s/dids/query", url.PathEscape(pid))
	var out []json.RawMessage

	res, err := c.http.R().SetContext(ctx).SetBody(map[string]any{}).SetResult(&out).Post(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	return out, nil
}

// AddServiceEndpoint binds a service endpoint to a participant's DID.
func (c *EdcClient) AddServiceEndpoint(ctx context.Context, pid string, did string, endpoint *ServiceEndpointDto) error {
	encodedDid := base64.RawURLEncoding.EncodeToString([]byte(did))
	path := fmt.Sprintf("/participants/%s/dids/%s/endpoints", url.PathEscape(pid), encodedDid)

	res, err := c.http.R().SetContext(ctx).SetBody(endpoint).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RemoveServiceEndpoint deletes a service endpoint from a participant's DID.
func (c *EdcClient) RemoveServiceEndpoint(ctx context.Context, pid string, did string, endpointID string) error {
	encodedDid := base64.RawURLEncoding.EncodeToString([]byte(did))
	path := fmt.Sprintf("/participants/%s/dids/%s/endpoints/%s", url.PathEscape(pid), encodedDid, url.PathEscape(endpointID))

	res, err := c.http.R().SetContext(ctx).Delete(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// ListCredentials lists stored verifiable credentials, optionally filtering by type.
func (c *EdcClient) ListCredentials(ctx context.Context, pid string, vcType string) ([]VerifiableCredentialResourceDto, error) {
	path := fmt.Sprintf("/participants/%s/credentials", url.PathEscape(pid))
	if vcType != "" {
		path = fmt.Sprintf("%s?type=%s", path, url.QueryEscape(vcType))
	}

	var out []VerifiableCredentialResourceDto
	res, err := c.http.R().SetContext(ctx).SetResult(&out).Get(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	return out, nil
}

// DeleteCredential deletes a stored credential by its identifier.
func (c *EdcClient) DeleteCredential(ctx context.Context, pid string, credID string) error {
	path := fmt.Sprintf("/participants/%s/credentials/%s", url.PathEscape(pid), url.PathEscape(credID))

	res, err := c.http.R().SetContext(ctx).Delete(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// StoreCredential saves a verifiable credential directly into the participant's storage.
func (c *EdcClient) StoreCredential(ctx context.Context, pid string, cred *StoreCredentialDto) error {
	path := fmt.Sprintf("/participants/%s/credentials", url.PathEscape(pid))

	res, err := c.http.R().SetContext(ctx).SetBody(cred).Post(path)
	if err != nil {
		return fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return statusError(res.StatusCode(), path, res.Bytes())
	}

	return nil
}

// RequestDcpCredential initiates an asynchronous DCP credential request.
func (c *EdcClient) RequestDcpCredential(ctx context.Context, pid string, req *DcpCredentialRequestDto) (string, error) {
	path := fmt.Sprintf("/participants/%s/credentials/request", url.PathEscape(pid))
	var out struct {
		RequestID string `json:"requestId"`
	}

	res, err := c.http.R().SetContext(ctx).SetBody(req).SetResult(&out).Post(path)
	if err != nil {
		return "", fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return "", statusError(res.StatusCode(), path, res.Bytes())
	}

	return out.RequestID, nil
}

// GetDcpRequestStatus checks the progress of an asynchronous DCP credential request.
func (c *EdcClient) GetDcpRequestStatus(ctx context.Context, pid string, reqID string) (*DcpRequestStatusDto, error) {
	path := fmt.Sprintf("/participants/%s/credentials/request/%s", url.PathEscape(pid), url.PathEscape(reqID))
	var out DcpRequestStatusDto

	res, err := c.http.R().SetContext(ctx).SetResult(&out).Get(path)
	if err != nil {
		return nil, fmt.Errorf("identityhub: calling %s: %w", path, err)
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsStatusFailure() {
		return nil, statusError(res.StatusCode(), path, res.Bytes())
	}

	return &out, nil
}

// statusError translates non-2xx HTTP status codes into domain error sentinels.
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

	return common.NewUpstreamError("IdentityHub", status, path, body, sentinel)
}
