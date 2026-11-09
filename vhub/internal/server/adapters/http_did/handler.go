// Driving adapter serving the hub DID document at /.well-known/did.json, generated from Signer.PublishedKeys().
// One process publishes keys and signs, so a key is always published before it signs (Q15 proposal, REQ-HUB-010, REQ-HUB-015).
package http_did

import (
	"encoding/json"
	"net/http"

	"github.com/ging/alexandria/vhub/internal/server/adapters/httpx"
	"github.com/ging/alexandria/vhub/internal/server/app"
)

// Handler serves the DID document.
type Handler struct{ Listing *app.Listing }

// Register mounts the DID route on mux.
func (h Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /.well-known/did.json", func(w http.ResponseWriter, r *http.Request) {
		b, _ := json.Marshal(h.Listing.DIDDocument())
		w.Header().Set("Cache-Control", "no-cache")
		httpx.JSON(w, http.StatusOK, b)
	})
}
