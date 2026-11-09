// Edge: the hot path. Routes a message, classifies each gate, combines actions, forwards or rejects, and captures agreements.
// Built without HubAPI or DIDResolver, and it never rewrites a forwarded body (REQ-SYS-001..003, REQ-CLI-010..015, REQ-CLI-021, REQ-CLI-022).
package app

import (
	"encoding/json"
	"net/http"

	"github.com/ging/alexandria/vhub/internal/client/domain"
	"github.com/ging/alexandria/vhub/internal/core/codes"
	"github.com/ging/alexandria/vhub/internal/core/lifecycle"
	"github.com/ging/alexandria/vhub/internal/core/problem"
	"github.com/ging/alexandria/vhub/internal/core/semver"
	"github.com/ging/alexandria/vhub/internal/core/timex"
)

// Edge classifies and forwards traffic.
type Edge struct {
	Cell             *Cell
	Clock            timex.Clock
	Evaluator        Evaluator
	Upstream         Upstream
	Agreements       *Agreements
	Audit            *Audit
	Events           EventSink
	Metrics          Metrics
	Routes           []domain.Route
	DefaultAction    string
	UnknownAgreement string
	Space            string
}

// outcome is the evaluation of one gate.
type outcome struct {
	gate   domain.Gate
	mode   domain.Mode
	class  domain.Classification
	action domain.Action
}

// Handle processes one message end to end.
func (e *Edge) Handle(req *Request) *Response {
	route, ok := domain.Find(e.Routes, req.Listener, req.Method, req.Path)
	if !ok {
		if e.DefaultAction == "reject" {
			return e.defaultReject(req)
		}
		return e.forward(req)
	}
	snap, tL := e.Cell.Load(), e.Clock.Now()
	state := domain.StateAt(snap, tL)
	outs := e.classify(route, req.Body, snap, state, tL)
	var enforced []domain.Action
	var items []domain.Item
	for _, o := range outs {
		if o.mode == domain.Enforce {
			enforced = append(enforced, o.action)
			if o.class.Result != domain.OK {
				items = append(items, item(o))
			}
		}
		e.emitVerdict(req, route, state, tL, o)
	}
	final := domain.Combine(enforced)
	if final == domain.Reject {
		return e.reject(req, route, outs, items)
	}
	resp := e.forward(req)
	if final == domain.ForwardNotice {
		resp.Header.Set(domain.HeaderName, domain.Header(items))
	}
	if resp.Status >= 200 && resp.Status < 300 {
		if route.Message == "ContractAgreementMessage" {
			e.capture(req, route, outs, snap, tL)
		}
		if route.ValidateResponse != nil {
			e.validateResponse(req, route, resp, snap, state, tL)
		}
	}
	return resp
}

// classify evaluates every gate of the route; observe gates are not evaluated.
func (e *Edge) classify(route domain.Route, body []byte, snap *domain.Snapshot, state domain.State, tL timex.Instant) []outcome {
	mode := route.EffectiveMode()
	if mode == domain.Observe {
		return nil
	}
	selected, ok := domain.SelectBody(body, route.Select())
	var outs []outcome
	for _, g := range route.Gates {
		c := e.Gate(g, body, selected, !ok, snap, state, tL)
		outs = append(outs, outcome{gate: g, mode: mode, class: c, action: domain.Act(mode, c.Result)})
		version := ""
		if c.Matched != nil {
			version = *c.Matched
		}
		e.Metrics.Classified(string(g), version, string(c.Result))
	}
	return outs
}

// Gate classifies one gate for a message at tL.
func (e *Edge) Gate(g domain.Gate, body, selected []byte, unparseable bool, snap *domain.Snapshot, state domain.State, tL timex.Instant) domain.Classification {
	degraded := "continue"
	if snap != nil {
		degraded = snap.Params.DegradedMode(string(g))
	}
	if g == domain.GateAdmission {
		if c, ok := domain.PreCheck(state, degraded); ok {
			return c
		}
		return e.Agreements.Admit(body, snap, tL, e.UnknownAgreement)
	}
	in := domain.ClassifyInput{Snap: snap, State: state, Degraded: degraded, TL: tL, Unparseable: unparseable}
	return domain.Classify(in, func(v lifecycle.Record) domain.EvalResult {
		return e.Evaluator.Evaluate(selected, g, snap.Package(v))
	})
}

// item converts a gate outcome into a VHub-Verdict item.
func item(o outcome) domain.Item {
	it := domain.Item{Result: o.class.Result, Gate: o.gate, Code: o.class.Code(), Sunset: o.class.Sunset}
	if o.class.Matched != nil {
		it.Version = *o.class.Matched
	}
	return it
}

// emitVerdict emits the vhub.verdict event of a gate when the mode and classification require one.
func (e *Edge) emitVerdict(req *Request, route domain.Route, state domain.State, tL timex.Instant, o outcome) {
	if !domain.Emits(o.mode, o.class) {
		return
	}
	v := Verdict{
		Type: EvVerdict, ID: uuid(), TS: tL.String(), Space: e.Space, Listener: string(req.Listener), Route: route.Name(),
		Message: route.Message, Gate: string(o.gate), Mode: string(o.mode), State: string(state),
		Classification: string(o.class.Result), Action: string(o.action), Evaluated: o.class.Evaluated, Matched: o.class.Matched,
		Codes: codeStrings(o.class.Codes), Findings: o.class.Findings,
	}
	if v.Evaluated == nil {
		v.Evaluated = []string{}
	}
	if v.Findings == nil {
		v.Findings = []domain.Finding{}
	}
	if o.class.Sunset != nil {
		v.Sunset = o.class.Sunset.String()
	}
	e.Events.Emit(v)
}

// forward sends the request upstream and returns its response unchanged.
func (e *Edge) forward(req *Request) *Response {
	resp, err := e.Upstream.Forward(req)
	if err != nil {
		p := problem.Problem{Type: "about:blank", Title: "bad gateway", Status: http.StatusBadGateway, Detail: err.Error(), Severity: "error"}
		return &Response{Status: http.StatusBadGateway, Header: http.Header{"Content-Type": {problem.ContentType}}, Body: p.JSON()}
	}
	if resp.Header == nil {
		resp.Header = http.Header{}
	}
	return resp
}

// reject builds the §4.8 response for a rejected message.
func (e *Edge) reject(req *Request, route domain.Route, outs []outcome, items []domain.Item) *Response {
	var first domain.Classification
	for _, o := range outs {
		if o.action == domain.Reject {
			first = o.class
			break
		}
	}
	hdr := http.Header{}
	hdr.Set(domain.HeaderName, domain.Header(items))
	if req.Listener == domain.MgmtIngress {
		p := map[string]any{"type": problem.TypeURI(first.Code()), "title": string(first.Code()), "status": http.StatusUnprocessableEntity,
			"code": string(first.Code()), "severity": "error", "findings": nonNil(first.Findings)}
		hdr.Set("Content-Type", problem.ContentType)
		return &Response{Status: http.StatusUnprocessableEntity, Header: hdr, Body: marshal(p)}
	}
	reasons := []string{}
	for _, f := range first.Findings {
		reasons = append(reasons, f.Message)
	}
	if len(reasons) == 0 {
		reasons = append(reasons, string(first.Code()))
	}
	hdr.Set("Content-Type", "application/json")
	body := domain.DSPError(route.Message, first.Code(), reasons, domain.Field(req.Body, "providerPid", "dspace:providerPid"), domain.Field(req.Body, "consumerPid", "dspace:consumerPid"))
	return &Response{Status: http.StatusBadRequest, Header: hdr, Body: body}
}

// defaultReject answers a message that matches no route under default_action reject (agent decision on its body).
func (e *Edge) defaultReject(req *Request) *Response {
	status := http.StatusBadRequest
	if req.Listener == domain.MgmtIngress {
		status = http.StatusUnprocessableEntity
	}
	p := problem.From(codes.New(codes.RouteDefaultReject, "%s %s matches no route", req.Method, req.Path), status)
	return &Response{Status: status, Header: http.Header{"Content-Type": {problem.ContentType}}, Body: p.JSON()}
}

// capture registers an accepted ContractAgreementMessage with the version that passed (§4.9).
func (e *Edge) capture(req *Request, route domain.Route, outs []outcome, snap *domain.Snapshot, tL timex.Instant) {
	if snap == nil {
		return
	}
	source := string(req.Listener)
	// The registered version must pass every evaluated gate; a shadow violation leaves the message unverified.
	var version string
	for _, o := range outs {
		if o.class.Result == domain.Violation {
			version = ""
			break
		}
		if o.class.Matched != nil && (version == "" || semver.Less(semver.MustParse(*o.class.Matched), semver.MustParse(version))) {
			version = *o.class.Matched
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
	e.Agreements.Capture(req.Body, role, source, version, snap, tL)
}

// validateResponse buffers a catalog response and classifies it in shadow without blocking (§4.10).
func (e *Edge) validateResponse(req *Request, route domain.Route, resp *Response, snap *domain.Snapshot, state domain.State, tL timex.Instant) {
	e.Audit.Observe(resp.Body)
	mode := route.ValidateResponse.Mode
	if mode == "" {
		mode = domain.Shadow
	}
	if mode == domain.Observe {
		return
	}
	sel, ok := domain.SelectBody(resp.Body, "$")
	c := e.Gate(domain.GateCatalog, resp.Body, sel, !ok, snap, state, tL)
	e.emitVerdict(req, route, state, tL, outcome{gate: domain.GateCatalog, mode: mode, class: c, action: domain.Forward})
}

// nonNil returns an empty slice instead of nil for JSON.
func nonNil(f []domain.Finding) []domain.Finding {
	if f == nil {
		return []domain.Finding{}
	}
	return f
}

// marshal encodes a value as JSON.
func marshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}
