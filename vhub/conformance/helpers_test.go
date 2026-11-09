// Shared helpers of the conformance battery: JSON field access, coded error extraction and file naming.
// Kept small so each N-<CODE> case reads as its F5 description (F5 §5.2).
package conformance

import (
	"encoding/json"
	"strings"

	"github.com/ging/alexandria/vhub/internal/core/codes"
)

// jsonField returns a top-level field of a JSON object rendered as text ("" if absent).
func jsonField(b []byte, name string) string {
	var m map[string]json.RawMessage
	if json.Unmarshal(b, &m) != nil || m[name] == nil {
		return ""
	}
	var s string
	if json.Unmarshal(m[name], &s) == nil {
		return s
	}
	return string(m[name])
}

// codeOfErr extracts the catalog code of an error.
func codeOfErr(err error) string {
	if ce, ok := codes.As(err); ok {
		return string(ce.Code)
	}
	return ""
}

// trimExt removes the extension of a file name.
func trimExt(name string) string { return strings.TrimSuffix(name, ".json") }
