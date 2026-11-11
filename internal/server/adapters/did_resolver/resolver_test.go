// Tests DID resolution for the signing-key check: did:key is local and did:web is fetched only from localhost when allowed.
// Fixtures: an httptest server publishing a did:web document (DP-5, T-HUB-015).
package did_resolver

import (
	"crypto/ed25519"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/did"
)

// pub is a fixed test public key.
var pub = ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)

// webDoc serves the DID document of the httptest host and returns its DID.
func webDoc(t *testing.T) did.DID {
	t.Helper()
	var d did.DID
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(did.NewDocument(d, []did.PublicKey{{Kid: string(d) + "#key-1", Key: pub}}))
	}))
	t.Cleanup(s.Close)
	d = did.WebDID(strings.TrimPrefix(s.URL, "http://"))
	return d
}

func TestDIDKeyIsLocal(t *testing.T) {
	d, vm := did.KeyDID(pub)
	doc, err := (Resolver{}).Resolve(d)
	if err != nil || !doc.HasKid(vm) {
		t.Fatalf("got %v", err)
	}
}

func TestDIDWebNeedsInsecureOptionOnLocalhost(t *testing.T) {
	d := webDoc(t)
	if _, err := (Resolver{}).Resolve(d); err == nil {
		t.Fatal("http resolution allowed without the option")
	}
	doc, err := (Resolver{AllowInsecureLocalhost: true}).Resolve(d)
	if err != nil || !doc.HasKid(string(d)+"#key-1") {
		t.Fatalf("got %v", err)
	}
}
