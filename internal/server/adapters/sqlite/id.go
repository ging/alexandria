// Random identifiers for drafts stored in SQLite.
// Kept apart so the repository code stays focused on SQL (DP-2).
package sqlite

import (
	"crypto/rand"
	"encoding/hex"
)

// newID returns a random 16-hex-digit identifier.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
