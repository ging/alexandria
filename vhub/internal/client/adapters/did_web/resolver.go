// did:web resolver over HTTPS; plain HTTP is allowed only for localhost with did.allow_insecure_localhost (PoC).
// Implements the DIDResolver port used by HubTrust (§4.4.1, DP-5, REQ-CLI-027, PRE-05).
package did_web

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ging/alexandria/vhub/internal/client/app"
	"github.com/ging/alexandria/vhub/internal/core/did"
)

// Resolver fetches DID documents.
type Resolver struct {
	AllowInsecureLocalhost bool
	HTTP                   *http.Client
}

var _ app.DIDResolver = Resolver{}

// New builds a resolver with a bounded timeout.
func New(insecure bool) Resolver {
	return Resolver{AllowInsecureLocalhost: insecure, HTTP: &http.Client{Timeout: 10 * time.Second}}
}

// Resolve downloads and decodes the DID document of d.
func (r Resolver) Resolve(d did.DID) (did.Document, error) {
	u, err := did.WebURL(d, r.AllowInsecureLocalhost)
	if err != nil {
		return did.Document{}, err
	}
	resp, err := r.HTTP.Get(u)
	if err != nil {
		return did.Document{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return did.Document{}, fmt.Errorf("did_web: %s answered %d", u, resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return did.Document{}, err
	}
	var doc did.Document
	if err := json.Unmarshal(b, &doc); err != nil {
		return did.Document{}, err
	}
	if doc.ID != string(d) {
		return did.Document{}, fmt.Errorf("did_web: document id %s is not %s", doc.ID, d)
	}
	return doc, nil
}
