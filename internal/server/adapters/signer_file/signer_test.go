// Contract tests of the file signer: the key persists across restarts and its proofs verify with its did:key document.
// Fixtures: a temporary key file (T-HUB-010, REQ-HUB-015).
package signer_file

import (
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/vc"
)

// keyFile returns a fresh key path.
func keyFile(t *testing.T) string { return filepath.Join(t.TempDir(), "hub.key") }

func TestKeyPersistsAcrossLoads(t *testing.T) {
	p := keyFile(t)
	a, err := Load(p, "")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := Load(p, "")
	if !a.PublicKey().Equal(b.PublicKey()) {
		t.Fatal("key regenerated")
	}
}

func TestT_HUB_010_ProofVerifiesWithDIDKeyDocument(t *testing.T) {
	s, _ := Load(keyFile(t), "")
	signed, err := s.Sign([]byte(`{"@context":["https://www.w3.org/ns/credentials/v2","c"],"a":1}`), 0)
	if err != nil {
		t.Fatal(err)
	}
	doc, _ := did.ResolveKey(s.KeyDID())
	k, ok := doc.AssertionKey(s.VerificationMethod())
	if !ok || vc.Verify(signed, k.Key) != nil {
		t.Fatal("proof not verifiable from the did:key document")
	}
}
