// Contract tests of the file signer: the key persists across restarts and only published keys sign.
// Realise the signer side of REQ-HUB-015 and REQ-HUB-010.
package signer_file

import (
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/vhub/internal/core/did"
	"github.com/ging/alexandria/vhub/internal/core/sign"
)

func TestKeyPersistsAcrossLoads(t *testing.T) {
	p := filepath.Join(t.TempDir(), "hub.key")
	a, err := Load(p, "did:web:h")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Load(p, "did:web:h")
	if !a.pub.Key.Equal(b.pub.Key) {
		t.Fatal("key regenerated")
	}
}

func TestT_HUB_015_SignsOnlyWithPublishedKey(t *testing.T) {
	s, _ := Load(filepath.Join(t.TempDir(), "hub.key"), "did:web:h")
	sig, _ := s.Sign([]byte("m"))
	doc := did.NewDocument("did:web:h", s.PublishedKeys())
	key, ok := doc.AssertionKey(sig.Kid)
	if !ok || sign.Verify([]byte("m"), sig, key) != nil {
		t.Fatalf("kid %s not verifiable from the DID document", sig.Kid)
	}
}
