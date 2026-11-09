// Unit tests for DID documents and did:web URL derivation.
// Covers assertionMethod lookup and the insecure localhost option (DP-4, DP-5).
package did

import (
	"crypto/ed25519"
	"testing"
)

// key generates a deterministic Ed25519 public key for a kid.
func key(kid string) PublicKey {
	pub := ed25519.NewKeyFromSeed(make([]byte, 32)).Public().(ed25519.PublicKey)
	return PublicKey{Kid: kid, Key: pub}
}

func TestAssertionKeyFound(t *testing.T) {
	doc := NewDocument("did:web:hub.example.com", []PublicKey{key("did:web:hub.example.com#key-1")})
	if _, ok := doc.AssertionKey("did:web:hub.example.com#key-1"); !ok {
		t.Fatal("key not found")
	}
}

func TestAssertionKeyRequiresAssertionMethod(t *testing.T) {
	doc := NewDocument("did:web:hub.example.com", []PublicKey{key("did:web:hub.example.com#key-1")})
	doc.AssertionMethod = nil
	if _, ok := doc.AssertionKey("did:web:hub.example.com#key-1"); ok {
		t.Fatal("key outside assertionMethod accepted")
	}
}

func TestWebURL(t *testing.T) {
	cases := []struct {
		did      DID
		insecure bool
		want     string
	}{
		{"did:web:hub.example.com", false, "https://hub.example.com/.well-known/did.json"},
		{"did:web:hub.example.com:spaces:demo", false, "https://hub.example.com/spaces/demo/did.json"},
		{"did:web:localhost%3A8080", false, "https://localhost:8080/.well-known/did.json"},
		{"did:web:localhost%3A8080", true, "http://localhost:8080/.well-known/did.json"},
		{"did:web:hub.example.com", true, "https://hub.example.com/.well-known/did.json"},
	}
	for _, c := range cases {
		if got, err := WebURL(c.did, c.insecure); err != nil || got != c.want {
			t.Errorf("%s: got %s %v", c.did, got, err)
		}
	}
}
