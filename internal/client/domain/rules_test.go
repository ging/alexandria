// Tests the rule set: one negative change per RULE invariant, the select_rule corpus and the CRUD helpers.
// Fixtures: the Annex C rule set and small rule literals (T-CLI-028, T-CLI-030, T-DOC-005).
package domain

import (
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
)

// rule is a minimal valid rule on dsp-ingress.
func rule(id string, prio int) Rule {
	return Rule{ID: id, Priority: prio, Listener: DSPIngress, Match: Match{Method: "POST", Path: "/x"}, Message: "ContractRequestMessage", Verify: Verify{Gates: []Gate{GateDSP}}}
}

// codeOf returns the code of a coded error.
func codeOf(e *codes.Error) codes.Code {
	if e == nil {
		return ""
	}
	return e.Code
}

func TestT_DOC_005_SeedValidates(t *testing.T) {
	if err := seed(t).Check(false); err != nil {
		t.Fatal(err)
	}
}

func TestT_CLI_028_Rule1Schema(t *testing.T) {
	if _, err := ParseRule([]byte(`{"id":"a","priority":1,"listener":"dsp-ingress","match":{"method":"POST","path":"/x"},"message":"ContractRequestMessage","verify":{}}`)); codeOf(err) != codes.RulesSchema {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_028_Rule2DuplicateID(t *testing.T) {
	rs := seed(t).With(rule("contract-request", 999))
	if err := rs.Check(false); codeOf(err) != codes.RulesDuplicateID {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_028_Rule3DuplicatePriority(t *testing.T) {
	rs := (&Ruleset{}).With(rule("a", 100)).With(rule("b", 100))
	if err := rs.Check(false); codeOf(err) != codes.RulesDuplicatePriority {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_028_Rule3SamePriorityOtherListener(t *testing.T) {
	b := rule("b", 100)
	b.Listener = DSPEgress
	if err := (&Ruleset{}).With(rule("a", 100)).With(b).Check(false); err != nil {
		t.Fatal(err)
	}
}

func TestT_CLI_028_Rule4AdmissionOnContractRequest(t *testing.T) {
	r := rule("a", 1)
	r.Verify.Gates = []Gate{GateAdmission}
	if err := (&Ruleset{}).With(r).Check(false); codeOf(err) != codes.RulesGateMessageMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_028_Rule4ValidateResponseOnContract(t *testing.T) {
	r := rule("a", 1)
	r.ValidateResponse = &ValidateResponse{Gate: GateCatalog}
	if err := (&Ruleset{}).With(r).Check(false); codeOf(err) != codes.RulesGateMessageMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_028_Rule5ManagementUpstream(t *testing.T) {
	r := rule("a", 1)
	r.Listener, r.Message = MgmtIngress, "management"
	if err := (&Ruleset{}).With(r).Check(false); codeOf(err) != codes.RulesUpstreamMissing {
		t.Fatalf("got %v", err)
	}
	if err := (&Ruleset{}).With(r).Check(true); err != nil {
		t.Fatal(err)
	}
}

func TestT_CLI_030_SelectCorpus(t *testing.T) {
	rs := seed(t)
	cases := []struct {
		method, path, body, want string
	}{
		{"POST", "/negotiations/urn:p/request", `{"@type":"ContractRequestMessage"}`, "contract-request"},
		{"POST", "/negotiations/urn:p/request", `{"@type":"dspace:ContractRequestMessage"}`, "contract-request"},
		{"POST", "/negotiations/urn:p/request", `{"@type":"Other"}`, ""},
		{"POST", "/negotiations/request", `{}`, "contract-request-initial"},
		{"POST", "/transfers/request", `{}`, "transfer-request"},
		{"GET", "/catalog/datasets/ds-1", ``, "dataset-request"},
		{"POST", "/negotiations/urn:p/agreement/verification", `{}`, "agreement-verification"},
		{"POST", "/nothing", `{}`, ""},
		{"POST", "/negotiations//request", `{}`, ""},
	}
	for _, c := range cases {
		got := ""
		if s := rs.Select(DSPIngress, c.method, c.path, []byte(c.body)); s.Rule != nil {
			got = s.Rule.ID
		}
		if got != c.want {
			t.Errorf("%s %s %s: got %q want %q", c.method, c.path, c.body, got, c.want)
		}
	}
}

func TestT_CLI_030_UnparseableTakesFirstURLMatch(t *testing.T) {
	s := seed(t).Select(DSPIngress, "POST", "/negotiations/urn:p/request", []byte("{"))
	if s.Rule == nil || s.Rule.ID != "contract-request" || !s.Unparseable {
		t.Fatalf("got %+v", s)
	}
}

func TestT_CLI_030_LowestPriorityWins(t *testing.T) {
	a, b := rule("late", 20), rule("early", 10)
	if s := (&Ruleset{}).With(a).With(b).Select(DSPIngress, "POST", "/x", []byte(`{}`)); s.Rule.ID != "early" {
		t.Fatalf("got %s", s.Rule.ID)
	}
}

func TestPointerEscapes(t *testing.T) {
	doc, _ := Decode([]byte(`{"a/b":{"m~n":[1,2]}}`))
	if v, ok := Pointer(doc, "/a~1b/m~0n/1"); !ok || v != float64(2) {
		t.Fatalf("got %v %v", v, ok)
	}
}

func TestCRUDHelpers(t *testing.T) {
	rs := seed(t)
	n := len(rs.Rules)
	without, ok := rs.Without("contract-request")
	if !ok || len(without.Rules) != n-1 || len(rs.Rules) != n {
		t.Fatal("Without changed the original or removed nothing")
	}
	if _, ok := rs.Replacing("nope", rule("nope", 1)); ok {
		t.Fatal("replaced a missing rule")
	}
}
