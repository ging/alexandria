// Named-information identifiers (RFC 6920) with SHA-256, used to address packages and files.
// The URL form of a digest is its unpadded base64url value (§2.5, REQ-CLI-006, REQ-CLI-007).
package ni

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strings"
)

const prefix = "ni:///sha-256;"

// Digest is an ni: URI of the form ni:///sha-256;<base64url>.
type Digest string

// Sum returns the ni: digest of b.
func Sum(b []byte) Digest {
	h := sha256.Sum256(b)
	return Digest(prefix + base64.RawURLEncoding.EncodeToString(h[:]))
}

// HexSum returns the lowercase hexadecimal SHA-256 of b (used for ETag and blob paths).
func HexSum(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Parse validates an ni: URI carrying a 32-byte SHA-256 value.
func Parse(s string) (Digest, error) {
	d := Digest(s)
	if _, err := d.Bytes(); err != nil {
		return "", err
	}
	return d, nil
}

// FromSegment builds a digest from its base64url URL segment.
func FromSegment(seg string) (Digest, error) { return Parse(prefix + seg) }

// Segment returns the base64url value used in URLs.
func (d Digest) Segment() string { return strings.TrimPrefix(string(d), prefix) }

// Bytes decodes the raw 32-byte hash.
func (d Digest) Bytes() ([]byte, error) {
	if !strings.HasPrefix(string(d), prefix) {
		return nil, fmt.Errorf("ni: %q lacks prefix %s", d, prefix)
	}
	b, err := base64.RawURLEncoding.DecodeString(d.Segment())
	if err != nil || len(b) != sha256.Size {
		return nil, fmt.Errorf("ni: %q is not a SHA-256 digest", d)
	}
	return b, nil
}

// Hex returns the hexadecimal form of the hash, or an empty string if invalid.
func (d Digest) Hex() string {
	b, err := d.Bytes()
	if err != nil {
		return ""
	}
	return hex.EncodeToString(b)
}
