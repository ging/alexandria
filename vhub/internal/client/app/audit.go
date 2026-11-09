// Passive audit: keeps the last audit.window catalog responses and re-classifies them when a new version is verified.
// Classifies at new_content_from(X) and enforced_from(X) with margin 0 and makes no active request (§4.10, Q5, REQ-CLI-023).
package app

import (
	"sync"

	"github.com/ging/alexandria/vhub/internal/client/domain"
	"github.com/ging/alexandria/vhub/internal/core/codes"
	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/timex"
)

// Audit is the passive audit buffer and report.
type Audit struct {
	Window    int
	Evaluator Evaluator
	Events    EventSink
	Clock     timex.Clock
	Space     string
	mu        sync.Mutex
	responses [][]byte
	report    *AuditReport
}

// AuditEntry is one predicted outcome of a stored response.
type AuditEntry struct {
	Response       int           `json:"response"`
	At             timex.Instant `json:"at"`
	Classification string        `json:"classification"`
	Code           string        `json:"code"`
}

// AuditReport is GET /audit/reports/latest.
type AuditReport struct {
	GeneratedAt timex.Instant `json:"generated_at"`
	Version     string        `json:"version"`
	Entries     []AuditEntry  `json:"entries"`
}

// Observe stores a catalog response, keeping at most Window of them.
func (a *Audit) Observe(body []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.responses = append(a.responses, append([]byte(nil), body...))
	if len(a.responses) > a.Window {
		a.responses = a.responses[len(a.responses)-a.Window:]
	}
}

// OnNewVersion predicts how stored responses fare against the newly verified version x.
func (a *Audit) OnNewVersion(x lifecycle.Record, snap *domain.Snapshot) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ref := *snap
	ref.Params.LeaseMargin = 0
	rep := &AuditReport{GeneratedAt: a.Clock.Now(), Version: x.Version.String(), Entries: []AuditEntry{}}
	for i, body := range a.responses {
		sel, ok := domain.SelectBody(body, "$")
		for _, at := range []timex.Instant{x.NewContentFrom, x.EnforcedFrom} {
			c := domain.Classify(domain.ClassifyInput{Snap: &ref, State: domain.Active, TL: at, Unparseable: !ok}, func(v lifecycle.Record) domain.EvalResult {
				return a.Evaluator.Evaluate(sel, domain.GateCatalog, ref.Package(v))
			})
			code := codes.Code("")
			switch c.Result {
			case domain.Warning:
				code = codes.AuditWillWarn
			case domain.Violation:
				code = codes.AuditWillReject
			default:
				continue
			}
			rep.Entries = append(rep.Entries, AuditEntry{Response: i, At: at, Classification: string(c.Result), Code: string(code)})
			n := notice(EvAudit, a.Space, a.Clock.Now(), code, "response "+at.String())
			n.Version = x.Version.String()
			a.Events.Emit(n)
		}
	}
	a.report = rep
}

// Latest returns the last audit report, if any.
func (a *Audit) Latest() *AuditReport {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.report
}
