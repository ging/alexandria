// Package app defines the client events: vhub.verdict (Appendix A.4) and the notices vhub.sync, vhub.state, vhub.agreement, vhub.audit, vhub.rules and vhub.gc.
// Every non-ok gate evaluation and every info finding becomes one vhub.verdict (§4.12, REQ-CLI-021, REQ-DOC-004).
package app

import (
	"crypto/rand"
	"fmt"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/timex"
)

// Event types of §4.12.
const (
	EvVerdict   = "vhub.verdict"
	EvSync      = "vhub.sync"
	EvState     = "vhub.state"
	EvAgreement = "vhub.agreement"
	EvAudit     = "vhub.audit"
	EvRules     = "vhub.rules"
	EvGC        = "vhub.gc"
)

// Verdict is the vhub.verdict event of Appendix A.4.
type Verdict struct {
	Type           string           `json:"type"`
	ID             string           `json:"id"`
	TS             string           `json:"ts"`
	Namespace      string           `json:"namespace"`
	Listener       string           `json:"listener"`
	Rule           string           `json:"rule"`
	RulesRevision  int              `json:"rules_revision"`
	Message        string           `json:"message"`
	Gate           string           `json:"gate"`
	Mode           string           `json:"mode"`
	State          string           `json:"state"`
	Classification string           `json:"classification"`
	Action         string           `json:"action"`
	Evaluated      []string         `json:"evaluated"`
	Matched        *string          `json:"matched"`
	Sunset         string           `json:"sunset,omitempty"`
	Codes          []string         `json:"codes"`
	Findings       []domain.Finding `json:"findings"`
}

// Notice is the shape of the other event types (not normative).
type Notice struct {
	Type      string   `json:"type"`
	ID        string   `json:"id"`
	TS        string   `json:"ts"`
	Namespace string   `json:"namespace"`
	Code      string   `json:"code,omitempty"`
	Severity  string   `json:"severity"`
	Detail    string   `json:"detail,omitempty"`
	Version   string   `json:"version,omitempty"`
	Versions  []string `json:"versions,omitempty"`
	Agreement string   `json:"agreement,omitempty"`
	From      string   `json:"from,omitempty"`
	To        string   `json:"to,omitempty"`
	Revision  int      `json:"revision,omitempty"`
	Rules     []string `json:"rules,omitempty"`
}

// uuid returns a random RFC 4122 version-4 UUID.
func uuid() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// NewNotice builds a notice event for adapters.
func NewNotice(typ, ns string, t timex.Instant, c codes.Code, detail string) Notice {
	return notice(typ, ns, t, c, detail)
}

// notice builds a notice of a type and code at t.
func notice(typ, ns string, t timex.Instant, c codes.Code, detail string) Notice {
	sev := string(codes.SevInfo)
	if c != "" {
		sev = string(codes.SeverityOf(c))
	}
	return Notice{Type: typ, ID: uuid(), TS: t.String(), Namespace: ns, Code: string(c), Severity: sev, Detail: detail}
}

// codeStrings renders codes for events.
func codeStrings(cs []codes.Code) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, string(c))
	}
	return out
}
