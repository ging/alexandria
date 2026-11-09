// Package domain builds the VHub-Verdict header (RFC 9651 List of Tokens with parameters) and the DSP error bodies of §4.8.
// The header lists one item per enforce gate whose classification is not ok (REQ-CLI-013, DP-11).
package domain

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/timex"
)

// HeaderName is the response header carrying the verdict.
const HeaderName = "VHub-Verdict"

// Item is one member of the VHub-Verdict list.
type Item struct {
	Result  Result
	Gate    Gate
	Code    codes.Code
	Version string
	Sunset  *timex.Instant
}

// sfString serializes an RFC 9651 String.
func sfString(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Header serializes the list; version and sunset are only present on warnings.
func Header(items []Item) string {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		s := string(it.Result) + ";gate=" + string(it.Gate) + ";code=" + sfString(string(it.Code))
		if it.Result == Warning && it.Version != "" {
			s += ";version=" + sfString(it.Version)
		}
		if it.Result == Warning && it.Sunset != nil {
			s += ";sunset=@" + strconv.FormatInt(int64(*it.Sunset), 10)
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

// ErrorType returns the DSP error type of a message type.
func ErrorType(message string) string {
	switch {
	case message == "CatalogRequestMessage" || message == "DatasetRequestMessage":
		return "CatalogError"
	case strings.HasPrefix(message, "Contract"):
		return "ContractNegotiationError"
	case strings.HasPrefix(message, "Transfer"):
		return "TransferError"
	}
	return ""
}

// DSPError builds the reject body; providerPid and consumerPid are omitted when the input lacks them (Q4, DP-11).
func DSPError(message string, code codes.Code, reasons []string, providerPid, consumerPid string) []byte {
	body := map[string]any{
		"@context": []string{"https://w3id.org/dspace/2025/1/context.jsonld"},
		"@type":    ErrorType(message),
		"code":     string(code),
		"reason":   reasons,
	}
	if providerPid != "" {
		body["providerPid"] = providerPid
	}
	if consumerPid != "" {
		body["consumerPid"] = consumerPid
	}
	b, _ := json.Marshal(body)
	return b
}
