// Unit tests for the signed envelope and Ed25519 verification over JCS.
// Covers verification of the received bytes and tampering detection (REQ-SYS-009).
package sign

import (
	"crypto/ed25519"
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/vhub/internal/core/did"
)

// fixture signs a small document and returns its envelope bytes and the key.
func fixture(t *testing.T) ([]byte, did.PublicKey) {
	t.Helper()
	priv := ed25519.NewKeyFromSeed(make([]byte, 32))
	pk := did.PublicKey{Kid: "did:web:h#k", Key: priv.Public().(ed25519.PublicKey)}
	signed := map[string]any{"b": 1, "a": "x"}
	msg, _ := SigningInput(signed)
	env := Envelope[map[string]any]{Signed: signed, Signature: Signature{Kid: pk.Kid, Alg: AlgEdDSA, Sig: Encode(ed25519.Sign(priv, msg))}}
	b, _ := json.Marshal(env)
	return b, pk
}

func TestVerifyRawEnvelope(t *testing.T) {
	b, pk := fixture(t)
	raw, err := ParseRaw(b)
	if err != nil {
		t.Fatal(err)
	}
	msg, _ := raw.Input()
	if err := Verify(msg, raw.Signature, pk); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyDetectsTampering(t *testing.T) {
	b, pk := fixture(t)
	raw, _ := ParseRaw([]byte(string(b[:len(b)-0])))
	raw.Signed = json.RawMessage(`{"a":"y","b":1}`)
	msg, _ := raw.Input()
	if err := Verify(msg, raw.Signature, pk); err == nil {
		t.Fatal("tampered document verified")
	}
}

func TestVerifyRejectsOtherKid(t *testing.T) {
	b, pk := fixture(t)
	raw, _ := ParseRaw(b)
	msg, _ := raw.Input()
	pk.Kid = "did:web:h#other"
	if err := Verify(msg, raw.Signature, pk); err == nil {
		t.Fatal("kid mismatch accepted")
	}
}
