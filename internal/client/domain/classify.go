// Normative classification pseudocode of §4.7 as a pure function; evaluation of one version is injected.
// Evaluates the head first and older versions only if needed, so it is deterministic and cheap (REQ-SYS-001, REQ-SYS-007, REQ-CLI-011).
package domain

import (
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/timex"
)

// Result is ok, warning or violation.
type Result string

// Classifications of §2.4.
const (
	OK        Result = "ok"
	Warning   Result = "warning"
	Violation Result = "violation"
)

// Finding is one validation finding; its JSON form follows the A.4 findings items.
type Finding struct {
	Slot     string     `json:"slot"`
	Source   string     `json:"source"`
	Severity string     `json:"severity"`
	Focus    string     `json:"focus,omitempty"`
	Path     string     `json:"path,omitempty"`
	Message  string     `json:"message"`
	Code     codes.Code `json:"-"`
}

// EvalResult is evaluate(m, g, v) of §4.7.
type EvalResult struct {
	Pass                  bool
	Findings              []Finding
	RecommendedViolations bool
	ShaclWarnings         bool
}

// Classification is the result of a gate for a message.
type Classification struct {
	Result    Result
	Codes     []codes.Code
	Matched   *string
	Sunset    *timex.Instant
	Evaluated []string
	Findings  []Finding
}

// Code returns the primary code of the classification.
func (c Classification) Code() codes.Code {
	if len(c.Codes) == 0 {
		return ""
	}
	return c.Codes[0]
}

// ClassifyInput is the hot-path input of Classify.
type ClassifyInput struct {
	Snap        *Snapshot
	State       State
	Degraded    string
	TL          timex.Instant
	Unparseable bool
}

// violation builds a violation with one code.
func violation(c codes.Code) Classification {
	return Classification{Result: Violation, Codes: []codes.Code{c}, Evaluated: []string{}}
}

// PreCheck applies the per-state rows of §4.5 shared by every gate.
func PreCheck(state State, degraded string) (Classification, bool) {
	switch {
	case state == Onboarding:
		return violation(codes.ClientNotReady), true
	case state == FailClosed:
		return violation(codes.ClientFailClosed), true
	case state == Degraded && degraded == "reject":
		return violation(codes.ClientDegradedReject), true
	}
	return Classification{}, false
}

// Classify runs the §4.7 pseudocode.
func Classify(in ClassifyInput, eval func(v lifecycle.Record) EvalResult) Classification {
	if c, ok := PreCheck(in.State, in.Degraded); ok {
		return c
	}
	vt := in.Snap.InForce(in.TL)
	if len(vt) == 0 {
		return violation(codes.NoVersionInForce)
	}
	if in.Unparseable {
		return violation(codes.MessageUnparseable)
	}
	head := vt[0]
	r := eval(head)
	evaluated := []string{head.Version.String()}
	if r.Pass {
		hv := head.Version.String()
		c := Classification{Result: OK, Matched: &hv, Evaluated: evaluated, Findings: r.Findings}
		if r.RecommendedViolations {
			c.Codes = append(c.Codes, codes.VerdictRecommendedNonconformant)
		}
		if r.ShaclWarnings {
			c.Codes = append(c.Codes, codes.VerdictShaclWarning)
		}
		return c
	}
	for _, v := range vt[1:] {
		evaluated = append(evaluated, v.Version.String())
		if eval(v).Pass {
			mv := v.Version.String()
			c := Classification{Result: Warning, Codes: []codes.Code{codes.VerdictNotInHead}, Matched: &mv, Evaluated: evaluated, Findings: r.Findings}
			if end := lifecycle.End(in.Snap.Records, v.Version); end != timex.Never {
				c.Sunset = &end
			}
			return c
		}
	}
	code := codes.VerdictViolation
	for _, f := range r.Findings {
		if f.Code == codes.ContextNotInPackage {
			code = codes.ContextNotInPackage
		}
	}
	return Classification{Result: Violation, Codes: []codes.Code{code}, Evaluated: evaluated, Findings: r.Findings}
}
