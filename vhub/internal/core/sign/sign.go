// Signed envelope {signed, signature{kid, alg, sig}} and Ed25519 verification over JCS(signed).
// The same code checks listings, version records and manifest.sig on both sides (§3.4, REQ-SYS-009, REQ-HUB-010).
package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ging/alexandria/vhub/internal/core/canon"
	"github.com/ging/alexandria/vhub/internal/core/did"
)

// AlgEdDSA is the only signature algorithm of v1.
const AlgEdDSA = "EdDSA"

// Signature is the {kid, alg, sig} object of §3.4.
type Signature struct {
	Kid string `json:"kid"`
	Alg string `json:"alg"`
	Sig string `json:"sig"`
}

// Envelope is a typed signed document.
type Envelope[T any] struct {
	Signed    T         `json:"signed"`
	Signature Signature `json:"signature"`
}

// Raw is an envelope whose signed part is kept as received, so verification never re-serializes it.
type Raw struct {
	Signed    json.RawMessage `json:"signed"`
	Signature Signature       `json:"signature"`
}

// ErrInvalid reports a signature that does not verify.
var ErrInvalid = errors.New("sign: signature does not verify")

// SigningInput returns JCS(signed), the bytes that are signed.
func SigningInput(signed any) ([]byte, error) { return canon.Marshal(signed) }

// Encode renders raw signature bytes as unpadded base64url.
func Encode(sig []byte) string { return base64.RawURLEncoding.EncodeToString(sig) }

// Verify checks sig over msg with key.
func Verify(msg []byte, s Signature, key did.PublicKey) error {
	if s.Alg != AlgEdDSA || s.Kid != key.Kid {
		return ErrInvalid
	}
	raw, err := base64.RawURLEncoding.DecodeString(s.Sig)
	if err != nil || !ed25519.Verify(key.Key, msg, raw) {
		return ErrInvalid
	}
	return nil
}

// ParseRaw decodes an envelope without touching its signed bytes.
func ParseRaw(b []byte) (Raw, error) {
	var r Raw
	if err := json.Unmarshal(b, &r); err != nil {
		return Raw{}, fmt.Errorf("sign: %w", err)
	}
	if len(r.Signed) == 0 {
		return Raw{}, errors.New("sign: envelope without signed")
	}
	return r, nil
}

// Input returns JCS of the signed part as received.
func (r Raw) Input() ([]byte, error) { return canon.Transform(r.Signed) }
