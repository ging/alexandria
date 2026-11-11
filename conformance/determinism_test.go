// Determinism tests of F5 §5.4 and document validation of Appendix A for everything the hub and the client emit.
// Fixtures: two independent client instances on the same corpus, the date table rows and an in-process F6 hub (T-SYS-001, T-CLI-030, T-HUB-005, T-HUB-010, T-DOC-001..006).
package conformance

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/json"
	"os"
	"testing"

	cdomain "github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/testkit"
	"github.com/mr-tron/base58"
)

// corpus is the (instant, action) grid of F6.3 plus an unparseable body.
var corpus = []string{"odrl:use", "ex:access", "odrl:read", "ex:other"}

// instants are the F6.3 instants at which both clients decide.
var instants = []string{"2026-10-02T12:00:00Z", "2026-10-10T12:00:00Z", "2026-11-05T12:00:00Z"}

func TestT_SYS_001_TwoClientsSameDecisions(t *testing.T) {
	s := f6Hub(t)
	a, b := testkit.NewClient(t, s), testkit.NewClient(t, s)
	for _, ts := range instants {
		s.Clock.Set(ts)
		_ = s.Hub.Listing.Tick(s.Clock.Now())
		a.SyncNow(t)
		b.SyncNow(t)
		for _, act := range corpus {
			req := cdomain.Request{Listener: cdomain.DSPIngress, Method: "POST", Path: testkit.ContractRequestPath, Body: testkit.ContractRequest(act)}
			x := a.Edge.Explain(appRequest(req))
			y := b.Edge.Explain(appRequest(req))
			xj, _ := json.Marshal(x)
			yj, _ := json.Marshal(y)
			if !bytes.Equal(xj, yj) {
				t.Fatalf("%s %s:\n%s\n%s", ts, act, xj, yj)
			}
		}
	}
}

func TestT_CLI_030_TwoParsesSelectTheSameRule(t *testing.T) {
	raw, _ := os.ReadFile("../spec/examples/rules.json")
	a, _ := cdomain.ParseRuleset(raw)
	b, _ := cdomain.ParseRuleset(raw)
	paths := []string{"/negotiations/request", "/negotiations/x/request", "/transfers/request", "/transfers/x/start", "/catalog/request", "/nothing"}
	for _, p := range paths {
		for _, body := range []string{`{"@type":"ContractRequestMessage"}`, `{`, ``} {
			x, y := a.Select(cdomain.DSPIngress, "POST", p, []byte(body)), b.Select(cdomain.DSPIngress, "POST", p, []byte(body))
			if (x.Rule == nil) != (y.Rule == nil) || (x.Rule != nil && x.Rule.ID != y.Rule.ID) || x.Unparseable != y.Unparseable {
				t.Fatalf("%s %q differ", p, body)
			}
		}
	}
}

func TestT_HUB_005_DatesAreDeterministic(t *testing.T) {
	recs := []lifecycle.Record{{Version: semver.MustParse("1.0.0"), Class: lifecycle.Initial, NewContentFrom: timex.MustInstant("2026-09-03T10:00:00Z"), EnforcedFrom: timex.MustInstant("2026-09-03T10:00:00Z")}}
	g := 10 * timex.Day
	rows := []lifecycle.Approval{
		{Version: semver.MustParse("1.1.0"), Class: lifecycle.Compatible},
		{Version: semver.MustParse("2.0.0"), Class: lifecycle.Breaking},
		{Version: semver.MustParse("2.0.0"), Class: lifecycle.Breaking, Grace: &g},
	}
	for _, a := range rows {
		a.P, a.Params = timex.MustInstant("2026-10-01T10:00:00Z"), lifecycle.Defaults()
		x, _ := lifecycle.Plan(a, recs)
		y, _ := lifecycle.Plan(a, recs)
		if x != y {
			t.Fatalf("%s %s: %+v vs %+v", a.Version, a.Class, x, y)
		}
	}
}

// independentVerify recomputes eddsa-jcs-2022 with a second code path: generic maps and a separate hash composition.
func independentVerify(secured []byte, pub ed25519.PublicKey) bool {
	var doc map[string]any
	if json.Unmarshal(secured, &doc) != nil {
		return false
	}
	proof, _ := doc["proof"].(map[string]any)
	sig, err := base58.Decode(proof["proofValue"].(string)[1:])
	if err != nil {
		return false
	}
	delete(doc, "proof")
	cfg := map[string]any{"@context": doc["@context"]}
	for k, v := range proof {
		if k != "proofValue" {
			cfg[k] = v
		}
	}
	c1, _ := canon.Marshal(cfg)
	c2, _ := canon.Marshal(doc)
	h1, h2 := sha256.Sum256(c1), sha256.Sum256(c2)
	return ed25519.Verify(pub, append(h1[:], h2[:]...), sig)
}

func TestT_HUB_010_ProofsVerifyIndependently(t *testing.T) {
	s := f6Hub(t)
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	rec, _ := s.Hub.Listing.Record(testkit.NS, "2.0.0")
	doc, _ := did.ResolveKey(s.DID)
	for _, b := range [][]byte{raw, rec} {
		p, _ := vc.ParseProof(b)
		k, ok := doc.AssertionKey(p.VerificationMethod)
		if !ok || !independentVerify(b, k.Key) || vc.Verify(b, k.Key) != nil {
			t.Fatal("a proof does not verify with both code paths")
		}
		var unsecured map[string]any
		_ = json.Unmarshal(b, &unsecured)
		delete(unsecured, "proof")
		x, _ := canon.Marshal(unsecured)
		y, _ := canon.Marshal(unsecured)
		if !bytes.Equal(x, y) {
			t.Fatal("two JCS serializations differ")
		}
	}
}

func TestT_DOC_001_002_006_EmittedCredentialsValidate(t *testing.T) {
	s := f6Hub(t)
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	if err := schemas.Validate(schemas.Versions, raw); err != nil {
		t.Errorf("A.1: %v", err)
	}
	rec, _ := s.Hub.Listing.Record(testkit.NS, "1.0.0")
	if err := schemas.Validate(schemas.Version, rec); err != nil {
		t.Errorf("A.2: %v", err)
	}
	c := testkit.VC[lifecycle.Record](t, rec)
	m, _ := s.Hub.Publication.Manifest(c.Subject.Manifest.Segment())
	if err := schemas.Validate(schemas.Manifest, m); err != nil {
		t.Errorf("A.7: %v", err)
	}
}

func TestT_DOC_003_DeployConfigValidates(t *testing.T) {
	b, _ := os.ReadFile("../deploy/client.json")
	if err := schemas.Validate(schemas.Config, b); err != nil {
		t.Fatal(err)
	}
}

func TestT_DOC_004_EmittedVerdictsValidate(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	for _, act := range corpus {
		c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, testkit.ContractRequest(act))
	}
	for _, v := range c.Events.Verdicts() {
		b, _ := json.Marshal(v)
		if err := schemas.Validate(schemas.Verdict, b); err != nil {
			t.Fatalf("%s: %v", b, err)
		}
	}
}

func TestT_DOC_005_ServedRuleSetValidates(t *testing.T) {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	c.SetMode(t, "negotiation-event", cdomain.Shadow)
	if err := schemas.Validate(schemas.Rules, c.Rules.Get().JSON()); err != nil {
		t.Fatal(err)
	}
}
