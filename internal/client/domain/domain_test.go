// Unit tests of the pure client domain: action matrix, route invariants, verdict header and body selection.
// Realise T-CLI-012 and parts of T-CLI-001 and T-CLI-013 (§4.3, §4.7, §4.8).
package domain

import (
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/timex"
)

// results and modes enumerate the matrix axes.
var (
	results = []Result{OK, Warning, Violation}
	modes   = []Mode{Observe, Shadow, Enforce}
)

// expected is the §4.7 mode table.
var expected = map[Mode]map[Result]Action{
	Observe: {OK: Forward, Warning: Forward, Violation: Forward},
	Shadow:  {OK: Forward, Warning: Forward, Violation: Forward},
	Enforce: {OK: Forward, Warning: ForwardNotice, Violation: Reject},
}

func TestT_CLI_012_ModeTimesClassification(t *testing.T) {
	for _, m := range modes {
		for _, r := range results {
			if got := Act(m, r); got != expected[m][r] {
				t.Errorf("%s/%s: got %s", m, r, got)
			}
		}
	}
}

func TestT_CLI_012_CombinationUpToFourGates(t *testing.T) {
	var walk func(n int, acts []Action)
	walk = func(n int, acts []Action) {
		if n == 0 {
			want := Forward
			for _, a := range acts {
				if a == Reject || (a == ForwardNotice && want == Forward) {
					want = a
				}
			}
			if got := Combine(acts); got != want {
				t.Errorf("%v: got %s want %s", acts, got, want)
			}
			return
		}
		for _, r := range results {
			walk(n-1, append(append([]Action{}, acts...), Act(Enforce, r)))
		}
	}
	for n := 0; n <= 4; n++ {
		walk(n, nil)
	}
}

func TestT_CLI_012_EventsOnlyOutsideObserve(t *testing.T) {
	v := Classification{Result: Violation, Codes: []codes.Code{codes.VerdictViolation}}
	info := Classification{Result: OK, Codes: []codes.Code{codes.VerdictRecommendedNonconformant}}
	if Emits(Observe, v) || !Emits(Shadow, v) || !Emits(Enforce, info) || Emits(Enforce, Classification{Result: OK}) {
		t.Fatal("event emission does not follow the mode table")
	}
}

func TestDefaultRoutesDoNotOverlap(t *testing.T) {
	rs := DefaultRoutes()
	if len(rs) != 16 || CheckOverlap(rs) != nil || CheckGateMessage(rs) != nil {
		t.Fatalf("annex routes invalid: %d", len(rs))
	}
}

func TestT_CLI_001_RouteInvariants(t *testing.T) {
	a := Route{Listener: DSPIngress, Match: Match{"POST", "/negotiations/:a/request"}, Message: "ContractRequestMessage", Gates: []Gate{GateDSP}}
	b := a
	b.Match.Path = "/negotiations/:b/request"
	if e := CheckOverlap([]Route{a, b}); e == nil || e.Code != codes.ConfigRouteOverlap {
		t.Fatalf("overlap: %v", e)
	}
	a.Gates = []Gate{GateAdmission}
	if e := CheckGateMessage([]Route{a}); e == nil || e.Code != codes.ConfigGateMessageMismatch {
		t.Fatalf("gate/message: %v", e)
	}
	m := Route{Listener: MgmtIngress, Match: Match{"POST", "/x"}, Message: "management", Gates: []Gate{GatePolicy}}
	if e := CheckUpstream([]Route{m}, false); e == nil || e.Code != codes.ConfigUpstreamMissing {
		t.Fatalf("upstream: %v", e)
	}
}

func TestT_CLI_013_HeaderSerialization(t *testing.T) {
	sunset := timex.Instant(1793613600)
	got := Header([]Item{
		{Result: Warning, Gate: GatePolicy, Code: codes.VerdictNotInHead, Version: "1.0.0", Sunset: &sunset},
		{Result: Violation, Gate: GateDSP, Code: codes.VerdictViolation, Version: "2.0.0"},
	})
	want := `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";version="1.0.0";sunset=@1793613600, violation;gate=dsp_message;code="VERDICT_VIOLATION"`
	if got != want {
		t.Fatalf("got %s", got)
	}
}

func TestSelectBody(t *testing.T) {
	body := []byte(`{"offer":{"permission":[{"action":"odrl:use"}]},"a":{"b c":1}}`)
	cases := map[string]string{"$": "", "$.offer.permission[0]": `{"action":"odrl:use"}`, "$['a']['b c']": "1"}
	for p, want := range cases {
		got, ok := SelectBody(body, p)
		if !ok || (want != "" && string(got) != want) {
			t.Errorf("%s: got %s %v", p, got, ok)
		}
	}
	for _, p := range []string{"$.missing", "$.offer.permission[3]", "$..action", "$.offer.permission[*]"} {
		if _, ok := SelectBody(body, p); ok {
			t.Errorf("%s selected a node", p)
		}
	}
}
