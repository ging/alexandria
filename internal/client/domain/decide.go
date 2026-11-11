// Package domain implements Decide: rule selection, per-gate classification, actions by mode, their combination and the response header.
// Handle and Explain both call it, so :explain decides exactly what the hot path decides (§4.3.2, §4.7, §4.8, REQ-CLI-012, REQ-CLI-022, REQ-CLI-032).
package domain

import (
	"encoding/json"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/timex"
)

// Request is a message as received on a listener; Body is never modified.
type Request struct {
	Listener Listener
	Method   string
	Path     string
	Body     []byte
}

// Env is everything a decision depends on: one snapshot, one rule set revision, the state and the local instant.
type Env struct {
	Snap             *Snapshot
	Rules            *Ruleset
	State            State
	TL               timex.Instant
	DefaultAction    string
	UnknownAgreement string
	Eval             func(g Gate, v lifecycle.Record, m Message) EvalResult
	Agreement        func(id string) (*Entry, bool)
}

// GateResult is the outcome of one gate; Evaluated is false in observe.
type GateResult struct {
	Gate      Gate
	Mode      Mode
	Evaluated bool
	Class     Classification
	Action    Action
}

// Decision is the full outcome for a message.
type Decision struct {
	Rule        *Rule
	Revision    int
	Unparseable bool
	Gates       []GateResult
	Action      Action
	Code        codes.Code
	Reasons     []string
	Header      string
	Doc         any
}

// Decide applies select_rule, classifies each gate and combines the actions.
func Decide(env Env, req Request) Decision {
	sel := env.Rules.Select(req.Listener, req.Method, req.Path, req.Body)
	d := Decision{Rule: sel.Rule, Revision: env.Rules.Revision, Unparseable: sel.Unparseable, Doc: sel.Doc, Action: Forward}
	if sel.Rule == nil {
		if env.DefaultAction == "reject" {
			d.Action, d.Code, d.Reasons = Reject, codes.RuleDefaultReject, []string{"no rule matches the request"}
		}
		return d
	}
	r := sel.Rule
	msg, unparseable := message(req.Body, sel, r.Verify.Select)
	var enforced []Action
	var items []Item
	for _, g := range r.Verify.Gates {
		gr := GateResult{Gate: g, Mode: r.EffectiveMode(), Action: Forward}
		if gr.Mode != Observe {
			gr.Evaluated = true
			gr.Class = classifyGate(env, g, msg, sel.Doc, unparseable)
			gr.Action = Act(gr.Mode, gr.Class.Result)
		}
		if gr.Mode == Enforce {
			enforced = append(enforced, gr.Action)
			if gr.Class.Result != OK {
				items = append(items, item(gr, r.ID))
			}
		}
		if gr.Action == Reject && d.Code == "" {
			d.Code, d.Reasons = gr.Class.Code(), reasons(gr.Class)
		}
		d.Gates = append(d.Gates, gr)
	}
	d.Action = Combine(enforced)
	d.Header = Header(items)
	return d
}

// message selects the node of verify.select; a missing node is unparseable (§4.3.2).
func message(body []byte, sel Selection, ptr string) (Message, bool) {
	if sel.Unparseable {
		return Message{Root: body}, true
	}
	node, ok := Pointer(sel.Doc, ptr)
	if !ok {
		return Message{Root: body}, true
	}
	b, _ := json.Marshal(node)
	return Message{Root: body, Selected: b}, false
}

// classifyGate runs the state rows, then the admission table or the §4.7 pseudocode.
func classifyGate(env Env, g Gate, m Message, doc any, unparseable bool) Classification {
	degraded := env.Snap.paramsDegraded(g)
	if c, ok := PreCheck(env.State, degraded); ok {
		return c
	}
	if g == GateAdmission {
		if unparseable {
			return violation(codes.MessageUnparseable)
		}
		id, _ := Field(doc, "agreementId", "dspace:agreementId")
		e, found := env.Agreement(id)
		if !found {
			e = nil
		}
		var rec lifecycle.Record
		if e != nil {
			rec, _ = env.Snap.KnownRecord(e.Version)
		}
		return Admit(e, rec, env.Snap.Params.LeaseMargin, env.TL, env.UnknownAgreement)
	}
	return Classify(ClassifyInput{Snap: env.Snap, State: env.State, Degraded: degraded, TL: env.TL, Unparseable: unparseable},
		func(v lifecycle.Record) EvalResult { return env.Eval(g, v, m) })
}

// paramsDegraded returns the degraded mode of a gate in the head lifecycle, tolerating a nil snapshot.
func (s *Snapshot) paramsDegraded(g Gate) string {
	if s == nil {
		return "continue"
	}
	return s.Params.DegradedMode(string(g))
}

// item builds the header item of a non-ok enforce gate.
func item(gr GateResult, rule string) Item {
	it := Item{Result: gr.Class.Result, Gate: gr.Gate, Code: gr.Class.Code(), Rule: rule}
	if gr.Class.Result == Warning && gr.Class.Matched != nil {
		it.Version, it.Sunset = *gr.Class.Matched, gr.Class.Sunset
	}
	return it
}

// reasons lists one entry per finding, or the codes when there are none.
func reasons(c Classification) []string {
	out := []string{}
	for _, f := range c.Findings {
		out = append(out, f.Message)
	}
	if len(out) == 0 {
		for _, cd := range c.Codes {
			out = append(out, string(cd))
		}
	}
	return out
}
