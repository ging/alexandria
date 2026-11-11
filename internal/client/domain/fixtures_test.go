// Shared fixtures of the client domain tests: the F6 snapshot, an evaluator by admitted actions and the Annex C rule set.
// Versions 1.0.0 and 2.0.0 admit the F6 actions; evaluation passes when the message action is admitted by that version (F6.3).
package domain

import (
	"os"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// at parses an instant.
func at(s string) timex.Instant { return timex.MustInstant(s) }

// admits maps each F6 version to its admitted actions.
var admits = map[string][]string{"1.0.0": {"odrl:use", "ex:access"}, "2.0.0": {"odrl:use", "odrl:read"}}

// f6Snapshot returns the snapshot of a listing valid until validUntil with both F6 versions.
func f6Snapshot(validUntil string) *Snapshot {
	recs := []lifecycle.Record{
		{Version: semver.MustParse("1.0.0"), Manifest: ni.Sum([]byte("1")), Class: lifecycle.Initial, PublishedAt: at("2026-09-01T10:00:00Z"), NewContentFrom: at("2026-09-03T10:00:00Z"), EnforcedFrom: at("2026-09-03T10:00:00Z")},
		{Version: semver.MustParse("2.0.0"), Manifest: ni.Sum([]byte("2")), Class: lifecycle.Breaking, PublishedAt: at("2026-10-01T10:00:00Z"), NewContentFrom: at("2026-10-03T10:00:00Z"), EnforcedFrom: at("2026-11-02T10:00:00Z")},
	}
	known := map[string]lifecycle.Record{}
	pkgs := map[ni.Digest]*pkgv.Package{}
	for _, r := range recs {
		known[r.Version.String()] = r
		pkgs[r.Manifest] = &pkgv.Package{Manifest: pkgv.Manifest{Version: r.Version.String(), Lifecycle: lifecycle.Defaults()}}
	}
	return &Snapshot{Records: recs, Known: known, Packages: pkgs, Params: lifecycle.Defaults(), ValidUntil: at(validUntil)}
}

// evalByAction passes a policy_intake evaluation when the selected action is admitted by the version; other gates always pass.
func evalByAction(g Gate, v lifecycle.Record, m Message) EvalResult {
	if g != GatePolicy {
		return EvalResult{Pass: true}
	}
	doc, _ := Decode(m.Selected)
	a, _ := Pointer(doc, "/action")
	for _, x := range admits[v.Version.String()] {
		if x == a {
			return EvalResult{Pass: true}
		}
	}
	return EvalResult{Findings: []Finding{{Slot: "policy", Source: "json-schema", Severity: "Violation", Message: "action not admitted"}}}
}

// seed loads the Annex C rule set (DP-10).
func seed(t *testing.T) *Ruleset {
	t.Helper()
	b, err := os.ReadFile("../../../spec/examples/rules.json")
	if err != nil {
		t.Fatal(err)
	}
	rs, cerr := ParseRuleset(b)
	if cerr != nil {
		t.Fatal(cerr)
	}
	return rs
}

// env builds a decision environment at tL in state st.
func env(t *testing.T, tL string, st State) Env {
	t.Helper()
	return Env{
		Snap: f6Snapshot("2026-12-01T00:00:00Z"), Rules: seed(t), State: st, TL: at(tL), DefaultAction: "forward", UnknownAgreement: "forward",
		Eval: evalByAction, Agreement: func(string) (*Entry, bool) { return nil, false },
	}
}

// contractRequest is a ContractRequestMessage using action.
func contractRequest(action string) []byte {
	return []byte(`{"@type":"ContractRequestMessage","providerPid":"urn:p","consumerPid":"urn:c","action":"` + action + `"}`)
}

// send decides an F6 contract request on the enforce rule.
func send(e Env, action string) Decision {
	return Decide(e, Request{Listener: DSPIngress, Method: "POST", Path: "/negotiations/urn:p/request", Body: contractRequest(action)})
}
