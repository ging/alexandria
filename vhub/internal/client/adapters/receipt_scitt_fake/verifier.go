// Verifier of the fake transparency receipts: checks the digest and an Ed25519 signature by a hub assertion key.
// NOT conformant with SCITT; it stands behind the ReceiptVerifier port until Q2/Q17 are decided (DP-8, REQ-CLI-006).
package receipt_scitt_fake

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/ging/alexandria/vhub/internal/client/app"
	"github.com/ging/alexandria/vhub/internal/core/canon"
	"github.com/ging/alexandria/vhub/internal/core/did"
	"github.com/ging/alexandria/vhub/internal/core/ni"
)

// receipt mirrors the document produced by scitt_fake.
type receipt struct {
	Digest       ni.Digest `json:"digest"`
	RegisteredAt string    `json:"registered_at"`
	LogID        string    `json:"log_id"`
	Sig          string    `json:"sig"`
}

// Verifier checks receipts.
type Verifier struct{}

var _ app.ReceiptVerifier = Verifier{}

// Verify accepts a receipt for d signed by any assertion key of the hub.
func (Verifier) Verify(d ni.Digest, b []byte, keys []did.PublicKey) error {
	var r receipt
	if err := json.Unmarshal(b, &r); err != nil {
		return err
	}
	if r.Digest != d {
		return errors.New("receipt is for another digest")
	}
	msg, err := canon.Marshal(map[string]any{"digest": r.Digest, "registered_at": r.RegisteredAt, "log_id": r.LogID})
	if err != nil {
		return err
	}
	sig, err := base64.RawURLEncoding.DecodeString(r.Sig)
	if err != nil {
		return err
	}
	for _, k := range keys {
		if ed25519.Verify(k.Key, msg, sig) {
			return nil
		}
	}
	return errors.New("receipt signature does not verify with the hub keys")
}
