// Package app implements the hot path: Explainer decides without effects and Edge adds forwarding (or a check for an external proxy), events, capture and response validation.
// Both run domain.Decide on one snapshot and one rule revision; neither holds a HubAPI or DIDResolver (REQ-SYS-002, REQ-SYS-003, REQ-CLI-010..015, REQ-CLI-021, REQ-CLI-032).
package app

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/problem"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Settings are the configuration values the hot path reads.
type Settings struct {
	Namespace        string
	DefaultAction    string
	UnknownAgreement string
}

// Explainer decides like the hot path; it is built without Upstream, AgreementRepo or EventSink (REQ-CLI-032).
type Explainer struct {
	Snap       *Cell
	Rules      *RulesCell
	Evaluator  Evaluator
	Agreements AgreementReader
	Clock      timex.Clock
	Settings   Settings
}

// decision runs Decide with one snapshot, one rule revision and the current local instant.
func (x *Explainer) decision(req *Request) (domain.Decision, domain.State, timex.Instant, *domain.Snapshot) {
	snap, rs, tL := x.Snap.Load(), x.Rules.Load(), x.Clock.Now()
	state := domain.StateAt(snap, tL)
	env := domain.Env{
		Snap: snap, Rules: rs, State: state, TL: tL, DefaultAction: x.Settings.DefaultAction, UnknownAgreement: x.Settings.UnknownAgreement,
		Eval: func(g domain.Gate, v lifecycle.Record, m domain.Message) domain.EvalResult {
			return x.Evaluator.Evaluate(m, g, snap.Package(v))
		},
		Agreement: func(id string) (*domain.Entry, bool) {
			e, ok, err := x.Agreements.Get(id)
			if err != nil || !ok || id == "" {
				return nil, false
			}
			return &e, true
		},
	}
	return domain.Decide(env, domain.Request{Listener: req.Listener, Method: req.Method, Path: req.Path, Body: req.Body}), state, tL, snap
}

// Explanation is the body of POST /rules:explain.
type Explanation struct {
	Rule     *string           `json:"rule"`
	Revision int               `json:"rules_revision"`
	State    string            `json:"state"`
	Gates    []GateExplanation `json:"gates"`
	Action   string            `json:"action"`
	Verdict  string            `json:"vhub_verdict,omitempty"`
	Code     string            `json:"code,omitempty"`
}

// GateExplanation is the classification of one gate in an explanation.
type GateExplanation struct {
	Gate           string           `json:"gate"`
	Mode           string           `json:"mode"`
	Evaluated      bool             `json:"evaluated"`
	Classification string           `json:"classification,omitempty"`
	Codes          []string         `json:"codes"`
	Matched        *string          `json:"matched"`
	Sunset         string           `json:"sunset,omitempty"`
	Action         string           `json:"action"`
	Findings       []domain.Finding `json:"findings"`
}

// Explain returns what the hot path would decide now, without any effect.
func (x *Explainer) Explain(req *Request) Explanation {
	d, state, _, _ := x.decision(req)
	out := Explanation{Revision: d.Revision, State: string(state), Action: string(d.Action), Verdict: d.Header, Code: string(d.Code), Gates: []GateExplanation{}}
	if d.Rule != nil {
		out.Rule = &d.Rule.ID
	}
	for _, g := range d.Gates {
		ge := GateExplanation{Gate: string(g.Gate), Mode: string(g.Mode), Evaluated: g.Evaluated, Codes: codeStrings(g.Class.Codes), Matched: g.Class.Matched, Action: string(g.Action), Findings: nonNil(g.Class.Findings)}
		if g.Evaluated {
			ge.Classification = string(g.Class.Result)
		}
		if g.Class.Sunset != nil {
			ge.Sunset = g.Class.Sunset.String()
		}
		out.Gates = append(out.Gates, ge)
	}
	return out
}

// Edge is the hot path with its effects.
type Edge struct {
	Explainer
	Upstream Upstream
	Capture  *Agreements
	Audit    *Audit
	Events   EventSink
	Metrics  Metrics
	Logger   *slog.Logger
}

// Handle processes one message end to end; the forwarded body and headers are never modified.
func (e *Edge) Handle(req *Request) *Response {
	d, state, tL, snap, rejected := e.admit(req)
	if rejected != nil {
		return rejected
	}
	resp := e.forward(req)
	if d.Action == domain.ForwardNotice {
		resp.Header.Set(domain.HeaderName, d.Header)
	}
	if d.Rule != nil && resp.Status >= 200 && resp.Status < 300 {
		if d.Rule.Message == "ContractAgreementMessage" {
			e.capture(req, d, snap, tL)
		}
		if d.Rule.ValidateResponse != nil {
			e.validateResponse(req, d, resp, snap, state, tL)
		}
	}
	return resp
}

// Check decides for an external proxy that forwards itself: 200 allows (with the notice header), anything else is the §4.8 reject.
func (e *Edge) Check(req *Request) *Response {
	d, _, tL, snap, rejected := e.admit(req)
	if rejected != nil {
		return rejected
	}
	resp := &Response{Status: http.StatusOK, Header: http.Header{}}
	if d.Action == domain.ForwardNotice {
		resp.Header.Set(domain.HeaderName, d.Header)
	}
	if d.Rule != nil && d.Rule.Message == "ContractAgreementMessage" {
		// The upstream answer is never seen here, so the agreement is captured when it is allowed.
		e.capture(req, d, snap, tL)
	}
	return resp
}

// admit decides, emits the gate verdicts and returns the reject response when the message must not pass.
func (e *Edge) admit(req *Request) (domain.Decision, domain.State, timex.Instant, *domain.Snapshot, *Response) {
	d, state, tL, snap := e.decision(req)
	for _, g := range d.Gates {
		e.emit(req, d, g, state, tL)
	}
	if d.Action != domain.Reject {
		return d, state, tL, snap, nil
	}
	ruleID := ""
	if d.Rule != nil {
		ruleID = d.Rule.ID
	}
	logx.Default(e.Logger).Warn("dsp message rejected",
		slog.String("listener", string(req.Listener)),
		slog.String("rule", ruleID),
		slog.String("state", string(state)),
		logx.Code(d.Code),
	)
	return d, state, tL, snap, e.reject(req, d)
}

// emit sends one vhub.verdict for an evaluated gate that is not ok or carries info codes, and counts it.
func (e *Edge) emit(req *Request, d domain.Decision, g domain.GateResult, state domain.State, tL timex.Instant) {
	if !g.Evaluated {
		return
	}
	version := ""
	if g.Class.Matched != nil {
		version = *g.Class.Matched
	}
	e.Metrics.Classified(d.Rule.ID, string(g.Gate), version, string(g.Class.Result))
	if !domain.Emits(g.Mode, g.Class) {
		return
	}
	v := Verdict{
		Type: EvVerdict, ID: uuid(), TS: tL.String(), Namespace: e.Settings.Namespace, Listener: string(req.Listener), Rule: d.Rule.ID,
		RulesRevision: d.Revision, Message: d.Rule.Message, Gate: string(g.Gate), Mode: string(g.Mode), State: string(state),
		Classification: string(g.Class.Result), Action: string(g.Action), Evaluated: g.Class.Evaluated, Matched: g.Class.Matched,
		Codes: codeStrings(g.Class.Codes), Findings: nonNil(g.Class.Findings),
	}
	if v.Evaluated == nil {
		v.Evaluated = []string{}
	}
	if g.Class.Sunset != nil {
		v.Sunset = g.Class.Sunset.String()
	}
	e.Events.Emit(v)
}

// forward sends the request upstream and returns its response unchanged.
func (e *Edge) forward(req *Request) *Response {
	resp, err := e.Upstream.Forward(req)
	if err != nil {
		logx.Default(e.Logger).Error("upstream forwarding failed", slog.String("listener", string(req.Listener)), logx.Err(err))
		p := problem.Problem{Type: "about:blank", Title: "bad gateway", Status: http.StatusBadGateway, Detail: err.Error(), Severity: "error"}
		return &Response{Status: http.StatusBadGateway, Header: http.Header{"Content-Type": {problem.ContentType}}, Body: p.JSON()}
	}
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp
}

// reject builds the §4.8 response; with no rule (default_action) it is a problem document (agent decision).
func (e *Edge) reject(req *Request, d domain.Decision) *Response {
	hdr := http.Header{}
	if d.Header != "" {
		hdr.Set(domain.HeaderName, d.Header)
	}
	if d.Rule == nil || d.Rule.Message == "management" {
		status := http.StatusUnprocessableEntity
		if d.Rule == nil && req.Listener != domain.MgmtIngress {
			status = http.StatusBadRequest
		}
		var findings []domain.Finding
		for _, g := range d.Gates {
			if g.Action == domain.Reject {
				findings = append(findings, g.Class.Findings...)
			}
		}
		p := map[string]any{
			"type": problem.TypeURI(d.Code), "title": string(d.Code), "status": status, "code": string(d.Code),
			"severity": "error", "findings": nonNil(findings),
		}
		hdr.Set("Content-Type", problem.ContentType)
		b, _ := json.Marshal(p)
		return &Response{Status: status, Header: hdr, Body: b}
	}
	pp, cp := pid(d.Doc, "providerPid"), pid(d.Doc, "consumerPid")
	hdr.Set("Content-Type", "application/json")
	return &Response{Status: http.StatusBadRequest, Header: hdr, Body: domain.DSPError(d.Rule.Message, d.Code, d.Reasons, pp, cp)}
}

// pid returns a pid of the input message, or nil when absent (Q4).
func pid(doc any, name string) *string {
	if v, ok := domain.Field(doc, name, "dspace:"+name); ok {
		return &v
	}
	return nil
}

// capture registers an accepted ContractAgreementMessage with the version that passed (§4.9).
func (e *Edge) capture(req *Request, d domain.Decision, snap *domain.Snapshot, tL timex.Instant) {
	if snap == nil {
		return
	}
	source := string(req.Listener)
	version := ""
	for _, g := range d.Gates {
		if !g.Evaluated || g.Class.Result == domain.Violation {
			version = ""
			break
		}
		if g.Class.Matched != nil && (version == "" || semver.Less(semver.MustParse(*g.Class.Matched), semver.MustParse(version))) {
			version = *g.Class.Matched
		}
	}
	if version == "" {
		vt := snap.InForce(tL)
		if len(vt) == 0 {
			return
		}
		version, source = vt[0].Version.String(), "unverified"
	}
	role := "consumer"
	if req.Listener == domain.DSPEgress {
		role = "provider"
	}
	e.Capture.Capture(req.Body, role, source, version, snap, tL)
}

// validateResponse buffers a catalog response and classifies it in shadow or observe without blocking (§4.11).
func (e *Edge) validateResponse(req *Request, d domain.Decision, resp *Response, snap *domain.Snapshot, state domain.State, tL timex.Instant) {
	e.Audit.Observe(resp.Body)
	mode := d.Rule.ResponseMode()
	if mode == domain.Observe {
		return
	}
	doc, ok := domain.Decode(resp.Body)
	c, pre := domain.PreCheck(state, "continue")
	if !pre {
		c = domain.Classify(domain.ClassifyInput{Snap: snap, State: state, TL: tL, Unparseable: !ok}, func(v lifecycle.Record) domain.EvalResult {
			b, _ := json.Marshal(doc)
			return e.Evaluator.Evaluate(domain.Message{Root: resp.Body, Selected: b}, domain.GateCatalog, snap.Package(v))
		})
	}
	e.emit(req, d, domain.GateResult{Gate: domain.GateCatalog, Mode: mode, Evaluated: true, Class: c, Action: domain.Forward}, state, tL)
}

// nonNil returns an empty slice for nil findings.
func nonNil(f []domain.Finding) []domain.Finding {
	if f == nil {
		return []domain.Finding{}
	}
	return f
}
