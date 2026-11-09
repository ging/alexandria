// Static bearer-token authentication: tokens.json maps a token to {principal, roles[]} (PoC substitute for OIDC).
// Produces the same Principal an auth_oidc adapter would, so the §3.6 role matrix applies unchanged (DP-3, REQ-HUB-017).
package auth_static

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	"github.com/ging/alexandria/vhub/internal/server/domain"
)

// Tokens maps bearer tokens to principals.
type Tokens map[string]domain.Principal

// Load reads a tokens.json file.
func Load(path string) (Tokens, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var t Tokens
	return t, json.Unmarshal(b, &t)
}

// Authenticate returns the principal of the request's bearer token.
func (t Tokens) Authenticate(r *http.Request) (domain.Principal, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return domain.Principal{}, false
	}
	p, ok := t[strings.TrimSpace(tok)]
	return p, ok
}
