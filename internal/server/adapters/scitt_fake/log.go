// Fake transparency log: Register returns a receipt {digest, registered_at, log_id, sig} signed with the hub key.
// NOT conformant with SCITT; it stands behind the TransparencyLog port until Q2/Q17 are decided (DP-8, REQ-HUB-004).
package scitt_fake

import (
	"encoding/json"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
)

// LogID identifies the fake log in receipts.
const LogID = "poc-log"

// Payload is the signed part of a receipt.
type Payload struct {
	Digest       ni.Digest     `json:"digest"`
	RegisteredAt timex.Instant `json:"registered_at"`
	LogID        string        `json:"log_id"`
}

// Receipt is the served manifest.receipt document.
type Receipt struct {
	Payload
	Sig string `json:"sig"`
}

// Log signs receipts with the hub signer.
type Log struct {
	Signer app.Signer
	Clock  timex.Clock
}

var _ app.TransparencyLog = Log{}

// Register records a digest and returns its receipt.
func (l Log) Register(d ni.Digest) ([]byte, error) {
	p := Payload{Digest: d, RegisteredAt: l.Clock.Now(), LogID: LogID}
	msg, err := canon.Marshal(p)
	if err != nil {
		return nil, err
	}
	sig, err := l.Signer.Sign(msg)
	if err != nil {
		return nil, err
	}
	return json.Marshal(Receipt{Payload: p, Sig: sig.Sig})
}
