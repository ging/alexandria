// Package domain implements the client rule set: parsing with RULE-1, the RULE-2..5 invariants, select_rule and CRUD as pure functions.
// Each change yields a new Ruleset validated as a whole; the caller swaps it atomically (§4.3.2, REQ-CLI-028, REQ-CLI-030, REQ-DOC-005).
package domain

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
)

// Match is the URL and body pattern of a rule.
type Match struct {
	Method string      `json:"method"`
	Path   string      `json:"path"`
	Body   []Condition `json:"body,omitempty"`
}

// Verify says which node is validated and with which gates.
type Verify struct {
	Select string `json:"select,omitempty"`
	Gates  []Gate `json:"gates"`
}

// ValidateResponse configures passive validation of catalog responses.
type ValidateResponse struct {
	Gate Gate `json:"gate"`
	Mode Mode `json:"mode,omitempty"`
}

// Rule is one entry of the rule set (Appendix A.6).
type Rule struct {
	ID               string            `json:"id"`
	Description      string            `json:"description,omitempty"`
	Priority         int               `json:"priority"`
	Listener         Listener          `json:"listener"`
	Match            Match             `json:"match"`
	Message          string            `json:"message"`
	Verify           Verify            `json:"verify"`
	Mode             Mode              `json:"mode,omitempty"`
	ValidateResponse *ValidateResponse `json:"validate_response,omitempty"`
}

// EffectiveMode returns the mode with its default observe.
func (r Rule) EffectiveMode() Mode {
	if r.Mode == "" {
		return Observe
	}
	return r.Mode
}

// ResponseMode returns the validate_response mode with its default shadow.
func (r Rule) ResponseMode() Mode {
	if r.ValidateResponse == nil || r.ValidateResponse.Mode == "" {
		return Shadow
	}
	return r.ValidateResponse.Mode
}

// Ruleset is the immutable rule set with its revision.
type Ruleset struct {
	Revision int    `json:"revision"`
	Rules    []Rule `json:"rules"`
}

// JSON serializes the rule set.
func (rs *Ruleset) JSON() []byte { b, _ := json.Marshal(rs); return b }

// ParseRuleset applies RULE-1 to raw bytes and decodes them.
func ParseRuleset(raw []byte) (*Ruleset, *codes.Error) {
	if err := schemas.Validate(schemas.Rules, raw); err != nil {
		return nil, codes.New(codes.RulesSchema, "%v", err)
	}
	var rs Ruleset
	if err := json.Unmarshal(raw, &rs); err != nil {
		return nil, codes.New(codes.RulesSchema, "%v", err)
	}
	if rs.Rules == nil {
		rs.Rules = []Rule{}
	}
	return &rs, nil
}

// ParseRule applies RULE-1 to one rule by wrapping it in a rule set.
func ParseRule(raw []byte) (Rule, *codes.Error) {
	rs, err := ParseRuleset([]byte(`{"revision":0,"rules":[` + string(raw) + `]}`))
	if err != nil {
		return Rule{}, err
	}
	if len(rs.Rules) != 1 {
		return Rule{}, codes.New(codes.RulesSchema, "expected exactly one rule")
	}
	return rs.Rules[0], nil
}

// Check applies RULE-1 (on the serialized set) and RULE-2..RULE-5 in order, reporting the first failure.
func (rs *Ruleset) Check(hasManagement bool) *codes.Error {
	if err := schemas.Validate(schemas.Rules, rs.JSON()); err != nil {
		return codes.New(codes.RulesSchema, "%v", err)
	}
	ids, prio := map[string]bool{}, map[string]bool{}
	for _, r := range rs.Rules {
		if ids[r.ID] {
			return codes.New(codes.RulesDuplicateID, "rule id %s is repeated", r.ID)
		}
		ids[r.ID] = true
	}
	for _, r := range rs.Rules {
		k := string(r.Listener) + "/" + strconv.Itoa(r.Priority)
		if prio[k] {
			return codes.New(codes.RulesDuplicatePriority, "%s has two rules with priority %d", r.Listener, r.Priority)
		}
		prio[k] = true
	}
	for _, r := range rs.Rules {
		for _, g := range r.Verify.Gates {
			if g == GateAdmission && r.Message != "TransferRequestMessage" {
				return codes.New(codes.RulesGateMessageMismatch, "rule %s uses agreement_admission with %s", r.ID, r.Message)
			}
		}
		if r.ValidateResponse != nil && r.Message != "CatalogRequestMessage" && r.Message != "DatasetRequestMessage" {
			return codes.New(codes.RulesGateMessageMismatch, "rule %s validates responses of %s", r.ID, r.Message)
		}
	}
	for _, r := range rs.Rules {
		if r.Listener == MgmtIngress && !hasManagement {
			return codes.New(codes.RulesUpstreamMissing, "rule %s uses mgmt-ingress without upstreams.management", r.ID)
		}
	}
	return nil
}

// Find returns the rule with id.
func (rs *Ruleset) Find(id string) (Rule, bool) {
	for _, r := range rs.Rules {
		if r.ID == id {
			return r, true
		}
	}
	return Rule{}, false
}

// With returns a copy with r added.
func (rs *Ruleset) With(r Rule) *Ruleset {
	return &Ruleset{Revision: rs.Revision, Rules: append(append([]Rule{}, rs.Rules...), r)}
}

// Replacing returns a copy with the rule id replaced by r, and whether id existed.
func (rs *Ruleset) Replacing(id string, r Rule) (*Ruleset, bool) {
	out := &Ruleset{Revision: rs.Revision, Rules: append([]Rule{}, rs.Rules...)}
	for i := range out.Rules {
		if out.Rules[i].ID == id {
			out.Rules[i] = r
			return out, true
		}
	}
	return out, false
}

// Without returns a copy without the rule id, and whether id existed.
func (rs *Ruleset) Without(id string) (*Ruleset, bool) {
	out := &Ruleset{Revision: rs.Revision, Rules: []Rule{}}
	found := false
	for _, r := range rs.Rules {
		if r.ID == id {
			found = true
			continue
		}
		out.Rules = append(out.Rules, r)
	}
	return out, found
}

// segments splits an absolute path into its segments, keeping empty ones.
func segments(p string) []string { return strings.Split(strings.TrimPrefix(p, "/"), "/") }

// pathMatches reports whether a level-1 URI template of full segments matches a path; a variable matches one non-empty segment.
func pathMatches(tpl, path string) bool {
	t, p := segments(tpl), segments(path)
	if len(t) != len(p) {
		return false
	}
	for i := range t {
		isVar := strings.HasPrefix(t[i], "{") && strings.HasSuffix(t[i], "}")
		if (isVar && p[i] == "") || (!isVar && t[i] != p[i]) {
			return false
		}
	}
	return true
}

// Selection is the outcome of select_rule.
type Selection struct {
	Rule        *Rule
	Unparseable bool
	Doc         any
}

// Select implements select_rule of §4.3.2.
func (rs *Ruleset) Select(l Listener, method, path string, body []byte) Selection {
	var cands []Rule
	for _, r := range rs.Rules {
		if r.Listener == l && r.Match.Method == method && pathMatches(r.Match.Path, path) {
			cands = append(cands, r)
		}
	}
	if len(cands) == 0 {
		return Selection{}
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].Priority < cands[j].Priority })
	doc, ok := Decode(body)
	if !ok {
		return Selection{Rule: &cands[0], Unparseable: true}
	}
	for i := range cands {
		all := true
		for _, c := range cands[i].Match.Body {
			all = all && c.Holds(doc)
		}
		if all {
			return Selection{Rule: &cands[i], Doc: doc}
		}
	}
	return Selection{}
}
