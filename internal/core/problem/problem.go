// Problem Details (RFC 9457) with the code, severity, path and line extensions of §3.6.
// Shared shape of every error body in /v1, the management API and the admin API (REQ-HUB-017, REQ-HUB-019).
package problem

import (
	"encoding/json"

	"github.com/ging/alexandria/internal/core/codes"
)

// ContentType is the media type of a problem document.
const ContentType = "application/problem+json"

// Problem is an RFC 9457 document; Findings carries one problem per finding when several exist.
type Problem struct {
	Type     string    `json:"type"`
	Title    string    `json:"title"`
	Status   int       `json:"status"`
	Detail   string    `json:"detail,omitempty"`
	Code     string    `json:"code,omitempty"`
	Severity string    `json:"severity"`
	Path     string    `json:"path,omitempty"`
	Line     int       `json:"line,omitempty"`
	Findings []Problem `json:"findings,omitempty"`
}

// TypeURI returns urn:eunomia:vhub:problem:<CODE>.
func TypeURI(c codes.Code) string { return "urn:eunomia:vhub:problem:" + string(c) }

// From builds a problem for a coded error with the given HTTP status.
func From(e *codes.Error, status int) Problem {
	return Problem{
		Type: TypeURI(e.Code), Title: string(e.Code), Status: status, Detail: e.Detail,
		Code: string(e.Code), Severity: string(codes.SeverityOf(e.Code)), Path: e.Path, Line: e.Line,
	}
}

// FromAll builds one problem whose first finding is the top level and lists all findings.
func FromAll(errs []*codes.Error, status int) Problem {
	p := From(errs[0], status)
	if len(errs) > 1 {
		for _, e := range errs {
			p.Findings = append(p.Findings, From(e, status))
		}
	}
	return p
}

// JSON serializes the problem.
func (p Problem) JSON() []byte {
	b, _ := json.Marshal(p)
	return b
}
