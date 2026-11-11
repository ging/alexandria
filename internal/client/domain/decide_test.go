// Tests classification, actions, states, admission and the response header against the F6 tables and the §4.5–§4.9 decision tables.
// Fixtures: the F6 snapshot, the Annex C rules with contract-request in enforce and an evaluator by admitted action (T-CLI-002, 008, 011–013, 016, 018, 022).
package domain

import (
	"encoding/json"
	"testing"

	"github.com/dunglas/httpsfv"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
)

// f6Rows is F6.3: instant → expected result of m1 (odrl:use), m2 (ex:access) and m3 (odrl:read).
var f6Rows = []struct {
	at         string
	m1, m2, m3 Result
}{
	{"2026-10-02T12:00:00Z", OK, OK, Violation},
	{"2026-10-03T09:59:55Z", OK, Warning, OK},
	{"2026-10-10T12:00:00Z", OK, Warning, OK},
	{"2026-11-02T10:00:05Z", OK, Warning, OK},
	{"2026-11-05T12:00:00Z", OK, Violation, OK},
}

// policy returns the policy_intake gate result of a decision.
func policy(d Decision) GateResult {
	for _, g := range d.Gates {
		if g.Gate == GatePolicy {
			return g
		}
	}
	return GateResult{}
}

func TestT_CLI_011_F6Table(t *testing.T) {
	for _, row := range f6Rows {
		e := env(t, row.at, Transition)
		for action, want := range map[string]Result{"odrl:use": row.m1, "ex:access": row.m2, "odrl:read": row.m3} {
			if got := policy(send(e, action)).Class.Result; got != want {
				t.Errorf("%s %s: got %s want %s", row.at, action, got, want)
			}
		}
	}
}

func TestT_CLI_013_F6HeaderOfM2(t *testing.T) {
	d := send(env(t, "2026-10-10T12:00:00Z", Transition), "ex:access")
	want := `warning;gate=policy_intake;code="VERDICT_NOT_IN_HEAD";rule="contract-request";version="1.0.0";sunset=@1793613600`
	if d.Header != want || d.Action != ForwardNotice {
		t.Fatalf("got %q %s", d.Header, d.Action)
	}
	if _, err := httpsfv.UnmarshalList([]string{d.Header}); err != nil {
		t.Fatalf("not an RFC 9651 list: %v", err)
	}
}

func TestT_CLI_011_ViolationRejectsWithHeadFindings(t *testing.T) {
	d := send(env(t, "2026-10-02T12:00:00Z", Active), "odrl:read")
	if d.Action != Reject || d.Code != codes.VerdictViolation || len(d.Reasons) != 1 {
		t.Fatalf("got %+v", d)
	}
}

func TestT_CLI_011_UnparseableBeforeNoVersion(t *testing.T) {
	e := env(t, "2026-09-01T00:00:00Z", Active)
	d := Decide(e, Request{Listener: DSPIngress, Method: "POST", Path: "/negotiations/urn:p/request", Body: []byte("{")})
	if g := policy(d); g.Class.Code() != codes.MessageUnparseable {
		t.Fatalf("got %v", g.Class.Codes)
	}
}

func TestT_CLI_011_NoVersionInForce(t *testing.T) {
	if g := policy(send(env(t, "2026-09-01T00:00:00Z", Active), "odrl:use")); g.Class.Code() != codes.NoVersionInForce {
		t.Fatalf("got %v", g.Class.Codes)
	}
}

func TestT_CLI_012_ModeMatrix(t *testing.T) {
	for _, m := range []Mode{Observe, Shadow, Enforce} {
		for _, c := range []struct {
			at, action string
			result     Result
		}{{"2026-10-10T12:00:00Z", "odrl:use", OK}, {"2026-10-10T12:00:00Z", "ex:access", Warning}, {"2026-11-05T12:00:00Z", "ex:access", Violation}} {
			e := env(t, c.at, Transition)
			rs, _ := e.Rules.Replacing("contract-request", withMode(t, e.Rules, m))
			e.Rules = rs
			d := send(e, c.action)
			want := Forward
			if m == Enforce {
				want = map[Result]Action{OK: Forward, Warning: ForwardNotice, Violation: Reject}[c.result]
			}
			if d.Action != want || (m == Observe && policy(d).Evaluated) {
				t.Errorf("%s/%s: action %s evaluated %v", m, c.result, d.Action, policy(d).Evaluated)
			}
		}
	}
}

// withMode returns the contract-request rule with mode m.
func withMode(t *testing.T, rs *Ruleset, m Mode) Rule {
	t.Helper()
	r, _ := rs.Find("contract-request")
	r.Mode = m
	return r
}

func TestT_CLI_012_CombinationIsMostRestrictive(t *testing.T) {
	if Combine([]Action{Forward, ForwardNotice, Reject}) != Reject || Combine([]Action{Forward, ForwardNotice}) != ForwardNotice || Combine(nil) != Forward {
		t.Fatal("combination order broken")
	}
}

func TestT_CLI_022_DefaultActionReject(t *testing.T) {
	e := env(t, "2026-10-10T12:00:00Z", Active)
	e.DefaultAction = "reject"
	d := Decide(e, Request{Listener: DSPIngress, Method: "POST", Path: "/nothing", Body: []byte(`{}`)})
	if d.Rule != nil || d.Action != Reject || d.Code != codes.RuleDefaultReject {
		t.Fatalf("got %+v", d)
	}
}

func TestT_CLI_022_DefaultActionForward(t *testing.T) {
	d := Decide(env(t, "2026-10-10T12:00:00Z", Active), Request{Listener: DSPIngress, Method: "POST", Path: "/nothing"})
	if d.Action != Forward || d.Code != "" {
		t.Fatalf("got %+v", d)
	}
}

func TestT_CLI_002_OnboardingIsNotReady(t *testing.T) {
	if g := policy(send(env(t, "2026-10-10T12:00:00Z", Onboarding), "odrl:use")); g.Class.Code() != codes.ClientNotReady {
		t.Fatalf("got %v", g.Class.Codes)
	}
}

func TestT_CLI_018_FailClosedAndDegradedReject(t *testing.T) {
	if g := policy(send(env(t, "2026-10-10T12:00:00Z", FailClosed), "odrl:use")); g.Class.Code() != codes.ClientFailClosed {
		t.Fatalf("fail closed: %v", g.Class.Codes)
	}
	e := env(t, "2026-10-10T12:00:00Z", Degraded)
	e.Snap.Params.Degraded = map[string]string{"policy_intake": "reject"}
	if g := policy(send(e, "odrl:use")); g.Class.Code() != codes.ClientDegradedReject {
		t.Fatalf("degraded: %v", g.Class.Codes)
	}
}

func TestT_CLI_018_DegradedContinueClassifies(t *testing.T) {
	if g := policy(send(env(t, "2026-10-10T12:00:00Z", Degraded), "ex:access")); g.Class.Result != Warning {
		t.Fatalf("got %v", g.Class.Result)
	}
}

// stateCells are the §4.5 transitions evaluated as StateAt(snapshot, t_l).
var stateCells = []struct {
	snap bool
	at   string
	want State
}{
	{false, "2026-10-10T12:00:00Z", Onboarding},
	{true, "2026-09-10T00:00:00Z", Active},
	{true, "2026-10-10T12:00:00Z", Transition},
	{true, "2026-12-01T00:00:00Z", Degraded},
	{true, "2026-12-07T23:59:59Z", Degraded},
	{true, "2026-12-08T00:00:00Z", FailClosed},
}

func TestT_CLI_008_StateCells(t *testing.T) {
	for _, c := range stateCells {
		var s *Snapshot
		if c.snap {
			s = f6Snapshot("2026-12-01T00:00:00Z")
		}
		if got := StateAt(s, at(c.at)); got != c.want {
			t.Errorf("%s: got %s want %s", c.at, got, c.want)
		}
	}
}

// entry is A1, registered with 1.0.0.
var entry = &Entry{AgreementID: "urn:A1", Version: "1.0.0"}

// revoked returns a 1.0.0 record revoked from 2026-11-12.
func revoked() lifecycle.Record {
	rf := at("2026-11-12T00:00:00Z")
	return lifecycle.Record{Version: semver.MustParse("1.0.0"), RevokedFrom: &rf}
}

func TestT_CLI_016_AdmissionRows(t *testing.T) {
	l := lifecycle.Defaults().LeaseMargin
	cases := []struct {
		e       *Entry
		at      string
		unknown string
		want    Result
		code    codes.Code
	}{
		{entry, "2026-11-12T00:00:05Z", "forward", OK, ""},
		{entry, "2026-11-12T00:00:10Z", "forward", Violation, codes.AgreementVersionRevoked},
		{nil, "2026-11-12T00:00:00Z", "forward", OK, codes.AgreementUnknown},
		{nil, "2026-11-12T00:00:00Z", "reject", Violation, codes.AgreementUnknown},
	}
	for i, c := range cases {
		got := Admit(c.e, revoked(), l, at(c.at), c.unknown)
		if got.Result != c.want || got.Code() != c.code {
			t.Errorf("row %d: got %s %v", i, got.Result, got.Codes)
		}
	}
}

func TestT_CLI_013_DSPErrorNullPids(t *testing.T) {
	p := "urn:p"
	var body map[string]any
	_ = json.Unmarshal(DSPError("ContractRequestMessage", codes.VerdictViolation, []string{"x"}, &p, nil), &body)
	if body["@type"] != "ContractNegotiationError" || body["providerPid"] != "urn:p" || body["consumerPid"] != nil {
		t.Fatalf("got %v", body)
	}
	if _, present := body["consumerPid"]; !present {
		t.Fatal("consumerPid omitted instead of null")
	}
}

func TestT_CLI_033_RetentionKeepsListedAndAgreed(t *testing.T) {
	s := f6Snapshot("2026-12-01T00:00:00Z")
	keep := Keep(s.Records[1:], []ni.Digest{s.Records[0].Manifest})
	if !keep[s.Records[0].Manifest] || !keep[s.Records[1].Manifest] || len(keep) != 2 {
		t.Fatalf("got %v", keep)
	}
	if k := Keep(s.Records[1:], nil); k[s.Records[0].Manifest] {
		t.Fatal("kept a version neither listed nor agreed")
	}
}
