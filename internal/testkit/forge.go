// Forgery helpers that re-sign mutated listings with the hub key, to build S4–S7 negative cases.
// The signatures are valid, so only the continuity and date checks can reject them (REQ-CLI-004).
package testkit

import (
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/server/app"
)

// LatestListing returns the hub's current listing envelope and its decoded content.
func (s *Server) LatestListing(t *testing.T) ([]byte, lifecycle.Listing) {
	t.Helper()
	raw, err := s.Hub.Listing.Latest(Member)
	if err != nil {
		t.Fatal(err)
	}
	var env sign.Envelope[lifecycle.Listing]
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return raw, env.Signed
}

// Forge returns the current listing mutated and re-signed with the active hub key.
func (s *Server) Forge(t *testing.T, mutate func(*lifecycle.Listing)) []byte {
	t.Helper()
	_, l := s.LatestListing(t)
	mutate(&l)
	b, err := app.Envelope(s.Signer, l)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Serve makes the fake hub answer with fixed bytes.
func Serve(b []byte) func([]byte) []byte { return func([]byte) []byte { return b } }
