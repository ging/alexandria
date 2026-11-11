// Package did_resolver resolves the hub DID for HubTrust: did:key by local computation and did:web over HTTPS.
// Plain HTTP is allowed only for localhost with did.allow_insecure_localhost (§4.4.1, DP-5, REQ-CLI-024, REQ-CLI-027, PRE-05).
package did_resolver

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/did"
)

// Resolver resolves DID documents.
type Resolver struct {
	AllowInsecureLocalhost bool
	HTTP                   *http.Client
}

var _ app.DIDResolver = Resolver{}

// New builds a resolver with a bounded timeout.
func New(insecure bool) Resolver {
	return Resolver{AllowInsecureLocalhost: insecure, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Resolve returns the DID document of d; did:key never touches the network.
func (r Resolver) Resolve(d did.DID) (did.Document, error) {
	if d.Method() == "key" {
		return did.ResolveKey(d)
	}
	u, err := did.WebURL(d, r.AllowInsecureLocalhost)
	if err != nil {
		return did.Document{}, err
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		return did.Document{}, err
	}
	resp, err := r.HTTP.Do(req)
	if err != nil {
		return did.Document{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return did.Document{}, fmt.Errorf("did_resolver: %s answered %d", u, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return did.Document{}, err
	}
	doc, err := did.Parse(b)
	if err == nil && doc.ID != string(d) {
		return did.Document{}, fmt.Errorf("did_resolver: document id %s is not %s", doc.ID, d)
	}
	return doc, err
}
