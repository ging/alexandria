// Package did_resolver resolves the hub DID for the signing-key check: did:key locally and did:web over HTTPS.
// http is used only for localhost when explicitly allowed (DP-5, REQ-HUB-015).
package did_resolver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/server/app"
)

// Resolver implements app.DIDResolver.
type Resolver struct {
	Client                 *http.Client
	AllowInsecureLocalhost bool
}

var _ app.DIDResolver = Resolver{}

// Resolve returns the DID document of d.
func (r Resolver) Resolve(d did.DID) (did.Document, error) {
	if d.Method() == "key" {
		return did.ResolveKey(d)
	}
	u, err := did.WebURL(d, r.AllowInsecureLocalhost)
	if err != nil {
		return did.Document{}, err
	}
	c := r.Client
	if c == nil {
		c = &http.Client{Timeout: 10 * time.Second}
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return did.Document{}, err
	}
	resp, err := c.Do(req) // #nosec G107 -- URL derived from the configured hub DID.
	if err != nil {
		return did.Document{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return did.Document{}, fmt.Errorf("did:web %s: HTTP %d", u, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return did.Document{}, err
	}
	doc, err := did.Parse(b)
	if err == nil && doc.ID != string(d) {
		return did.Document{}, fmt.Errorf("did:web %s: document id %s", u, doc.ID)
	}
	return doc, err
}
