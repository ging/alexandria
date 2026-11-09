// Tests for the date table of §3.3, the approval and revocation invariants, states (§3.5) and in-force sets (§4.6).
// Named T-* tests realise REQ-HUB-005, REQ-HUB-006, REQ-HUB-007, REQ-HUB-009, REQ-HUB-014, REQ-CLI-009 and REQ-SYS-007.
package lifecycle

import (
	"math/rand"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// at parses an instant fixture.
func at(s string) timex.Instant { return timex.MustInstant(s) }

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// f6 returns the two records of the F6 example.
func f6() []Record {
	return []Record{
		{Version: semver.MustParse("1.0.0"), Class: Initial, PublishedAt: at("2026-09-01T10:00:00Z"), NewContentFrom: at("2026-09-03T10:00:00Z"), EnforcedFrom: at("2026-09-03T10:00:00Z")},
		{Version: semver.MustParse("2.0.0"), Class: Breaking, PublishedAt: at("2026-10-01T10:00:00Z"), NewContentFrom: at("2026-10-03T10:00:00Z"), EnforcedFrom: at("2026-11-02T10:00:00Z")},
	}
}

// approval builds a default approval of version v with class c at P.
func approval(v string, c Class, p string) Approval {
	return Approval{Version: semver.MustParse(v), Class: c, P: at(p), Params: Defaults()}
}

// codeOf returns the code of a coded error or an empty code.
func codeOf(e *codes.Error) codes.Code {
	if e == nil {
		return ""
	}
	return e.Code
}

// versions renders an in-force set as version strings.
func versions(rs []Record) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Version.String())
	}
	return out
}

func TestT_HUB_005_Row1_InitialEnforcedAtNewContent(t *testing.T) {
	r, err := Plan(approval("1.0.0", Initial, "2026-09-01T10:00:00Z"), nil)
	if err != nil || r.NewContentFrom != at("2026-09-03T10:00:00Z") || r.EnforcedFrom != r.NewContentFrom {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row2_CompatibleInheritsOpenIGW(t *testing.T) {
	r, err := Plan(approval("2.1.0", Compatible, "2026-10-10T10:00:00Z"), f6())
	if err != nil || r.EnforcedFrom != at("2026-11-02T10:00:00Z") {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row2_CompatibleAfterIGWUsesNewContent(t *testing.T) {
	r, err := Plan(approval("2.1.0", Compatible, "2026-11-10T10:00:00Z"), f6())
	if err != nil || r.EnforcedFrom != at("2026-11-12T10:00:00Z") {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row3_GraceOnCompatible(t *testing.T) {
	a := approval("1.1.0", Compatible, "2026-09-10T10:00:00Z")
	a.Grace = ptr(10 * timex.Day)
	if _, err := Plan(a, f6()[:1]); codeOf(err) != codes.DatesGraceOnCompatible {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_005_Row4_EmergencyOnCompatible(t *testing.T) {
	a := approval("1.1.0", Compatible, "2026-09-10T10:00:00Z")
	a.Emergency, a.Grace = ptr(true), ptr(10*timex.Day)
	if _, err := Plan(a, f6()[:1]); codeOf(err) != codes.DatesEmergencyOnCompatible {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_005_Row5_BreakingDefaultGrace(t *testing.T) {
	r, err := Plan(approval("2.0.0", Breaking, "2026-10-01T10:00:00Z"), f6()[:1])
	if err != nil || r.EnforcedFrom != at("2026-11-02T10:00:00Z") {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row6_BreakingExplicitGrace(t *testing.T) {
	a := approval("2.0.0", Breaking, "2026-10-01T10:00:00Z")
	a.Grace = ptr(5 * timex.Day)
	if r, err := Plan(a, f6()[:1]); err != nil || r.EnforcedFrom != at("2026-10-08T10:00:00Z") {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row7_ZeroGraceWithoutEmergency(t *testing.T) {
	a := approval("2.0.0", Breaking, "2026-10-01T10:00:00Z")
	a.Grace = ptr(timex.Duration(0))
	if _, err := Plan(a, f6()[:1]); codeOf(err) != codes.DatesZeroGraceWithoutEmergency {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_005_Row8_EmergencyEnforcesAtNewContent(t *testing.T) {
	a := approval("2.0.0", Breaking, "2026-10-01T10:00:00Z")
	a.Emergency, a.Grace, a.Justification = ptr(true), ptr(timex.Duration(0)), "CVE"
	if r, err := Plan(a, f6()[:1]); err != nil || r.EnforcedFrom != r.NewContentFrom || !r.Emergency {
		t.Fatalf("got %+v %v", r, err)
	}
}

func TestT_HUB_005_Row8_EmergencyRequiresJustification(t *testing.T) {
	a := approval("2.0.0", Breaking, "2026-10-01T10:00:00Z")
	a.Emergency = ptr(true)
	if _, err := Plan(a, f6()[:1]); codeOf(err) != codes.DatesJustificationRequired {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_005_Row9_EmergencyWithGrace(t *testing.T) {
	a := approval("2.0.0", Breaking, "2026-10-01T10:00:00Z")
	a.Emergency, a.Grace, a.Justification = ptr(true), ptr(10*timex.Day), "x"
	if _, err := Plan(a, f6()[:1]); codeOf(err) != codes.DatesEmergencyWithGrace {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_006_Date1NonMonotonic(t *testing.T) {
	a := approval("3.0.0", Breaking, "2026-10-05T10:00:00Z")
	a.Grace = ptr(5 * timex.Day)
	x, _ := Plan(a, f6())
	if err := CheckApproval(x, f6(), Defaults()); codeOf(err) != codes.DatesNonMonotonic {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_006_Date2TooManyVersions(t *testing.T) {
	p := Defaults()
	p.MaxConcurrentVersions = 2
	a := approval("2.1.0", Compatible, "2026-10-05T10:00:00Z")
	x, _ := Plan(a, f6())
	if err := CheckApproval(x, f6(), p); codeOf(err) != codes.DatesTooManyVersions {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_006_F6ApprovalPasses(t *testing.T) {
	x, _ := Plan(approval("2.0.0", Breaking, "2026-10-01T10:00:00Z"), f6()[:1])
	if err := CheckApproval(x, f6()[:1], Defaults()); err != nil {
		t.Fatal(err)
	}
}

// revocation builds a revocation request of v at R with the default propagation window.
func revocation(v, r string) Revocation {
	return Revocation{Version: semver.MustParse(v), R: at(r), Justification: "superseded", PW: 2 * timex.Day}
}

func TestT_HUB_014_DefaultRevokedFrom(t *testing.T) {
	rf, err := CheckRevocation(revocation("1.0.0", "2026-11-10T00:00:00Z"), f6())
	if err != nil || rf != at("2026-11-12T00:00:00Z") {
		t.Fatalf("got %s %v", rf, err)
	}
}

func TestT_HUB_014_Rev1AlreadyRevoked(t *testing.T) {
	rs := f6()
	rs[0].RevokedFrom = ptr(at("2026-11-12T00:00:00Z"))
	if _, err := CheckRevocation(revocation("1.0.0", "2026-11-13T00:00:00Z"), rs); codeOf(err) != codes.DatesAlreadyRevoked {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_014_Rev2JustificationRequired(t *testing.T) {
	in := revocation("1.0.0", "2026-11-10T00:00:00Z")
	in.Justification = " "
	if _, err := CheckRevocation(in, f6()); codeOf(err) != codes.DatesJustificationRequired {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_014_Rev3TooEarly(t *testing.T) {
	in := revocation("1.0.0", "2026-11-10T00:00:00Z")
	in.RevokedFrom = ptr(at("2026-11-11T00:00:00Z"))
	if _, err := CheckRevocation(in, f6()); codeOf(err) != codes.DatesRevocationTooEarly {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_014_Rev4LeavesNoHead(t *testing.T) {
	if _, err := CheckRevocation(revocation("1.0.0", "2026-09-10T00:00:00Z"), f6()[:1]); codeOf(err) != codes.DatesRevokeLeavesNoHead {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_009_F6StatesOfFirstVersion(t *testing.T) {
	rs := f6()
	cases := map[string]State{
		"2026-09-02T00:00:00Z": Scheduled, "2026-10-03T09:59:59Z": Published, "2026-10-03T10:00:00Z": Deprecated,
		"2026-11-02T09:59:59Z": Deprecated, "2026-11-02T10:00:00Z": Retired,
	}
	for ts, want := range cases {
		if got := StateAt(rs, rs[0], at(ts)); got != want {
			t.Errorf("%s: got %s want %s", ts, got, want)
		}
	}
}

func TestT_HUB_009_MembershipFollowsStates(t *testing.T) {
	rs := f6()
	if got := versions(Members(rs, at("2026-11-01T00:00:00Z"))); len(got) != 2 {
		t.Fatalf("IGW: got %v", got)
	}
	if got := versions(Members(rs, at("2026-11-05T00:00:00Z"))); len(got) != 1 || got[0] != "2.0.0" {
		t.Fatalf("after retirement: got %v", got)
	}
	rs[0].RevokedFrom = ptr(at("2026-11-12T00:00:00Z"))
	if got := versions(Members(rs, at("2026-11-20T00:00:00Z"))); len(got) != 2 {
		t.Fatalf("revoked versions stay listed: got %v", got)
	}
}

func TestT_HUB_009_SupersededAfterCompatible(t *testing.T) {
	rs := []Record{
		{Version: semver.MustParse("1.0.0"), Class: Initial, NewContentFrom: at("2026-09-03T10:00:00Z"), EnforcedFrom: at("2026-09-03T10:00:00Z")},
		{Version: semver.MustParse("1.1.0"), Class: Compatible, NewContentFrom: at("2026-09-10T10:00:00Z"), EnforcedFrom: at("2026-09-10T10:00:00Z")},
	}
	if got := StateAt(rs, rs[0], at("2026-09-10T10:00:00Z")); got != Superseded {
		t.Fatalf("got %s", got)
	}
}

func TestT_HUB_009_RevokedRowWins(t *testing.T) {
	rs := f6()
	rs[1].RevokedFrom = ptr(at("2026-10-02T00:00:00Z"))
	if got := StateAt(rs, rs[1], at("2026-10-02T00:00:00Z")); got != Revoked {
		t.Fatalf("got %s", got)
	}
	if got := StateAt(rs, rs[0], at("2026-12-01T00:00:00Z")); got != Published {
		t.Fatalf("revoked successor before enforcement must not retire 1.0.0: got %s", got)
	}
}

func TestT_CLI_009_BoundariesWithLease(t *testing.T) {
	rs, l := f6(), 10*timex.Second
	cases := map[string]int{
		"2026-10-03T09:59:49Z": 1, "2026-10-03T09:59:50Z": 2, "2026-10-03T09:59:51Z": 2,
		"2026-11-02T10:00:09Z": 2, "2026-11-02T10:00:10Z": 1, "2026-11-02T10:00:11Z": 1,
	}
	for ts, want := range cases {
		if got := InForceSet(rs, at(ts), l); len(got) != want {
			t.Errorf("%s: got %v want %d versions", ts, versions(got), want)
		}
	}
}

func TestT_CLI_009_RevocationBoundaryWithLease(t *testing.T) {
	rs := f6()
	rs[1].RevokedFrom = ptr(at("2026-12-01T00:00:00Z"))
	if got := InForceSet(rs, at("2026-12-01T00:00:09Z"), 10); len(got) == 0 || got[0].Version.String() != "2.0.0" {
		t.Fatalf("revocation must apply after L: got %v", versions(got))
	}
	if got := InForceSet(rs, at("2026-12-01T00:00:10Z"), 10); len(got) != 0 {
		t.Fatalf("revocation not applied at rf+L: got %v", versions(got))
	}
}

func TestT_SYS_007_ContractOnlyAfterEnforcedPlusLease(t *testing.T) {
	rs := f6()
	for ts := at("2026-11-02T09:59:00Z"); ts < at("2026-11-02T10:01:00Z"); ts++ {
		contracted := len(InForceSet(rs, ts, 10)) == 1
		if contracted != (ts >= at("2026-11-02T10:00:10Z")) {
			t.Fatalf("contraction at %s is %v", ts, contracted)
		}
	}
}

func TestEndIsSunsetOfSuccessor(t *testing.T) {
	if got := End(f6(), semver.MustParse("1.0.0")); int64(got) != 1793613600 {
		t.Fatalf("got %d", got)
	}
	if End(f6(), semver.MustParse("2.0.0")) != timex.Never {
		t.Fatal("head must have no sunset")
	}
}

// randomRecords builds a plausible random history of n versions with monotonic dates.
func randomRecords(r *rand.Rand, n int) []Record {
	var out []Record
	t := at("2026-01-01T00:00:00Z")
	var lastEF timex.Instant
	for i := 0; i < n; i++ {
		t = t.Add(timex.Duration(r.Intn(40)+1) * timex.Day)
		nc := t.Add(2 * timex.Day)
		ef := nc.Add(timex.Duration(r.Intn(3)) * 10 * timex.Day)
		if ef < lastEF {
			ef = lastEF
		}
		lastEF = ef
		c := []Class{Compatible, Breaking}[r.Intn(2)]
		if i == 0 {
			c, ef = Initial, nc
		}
		rec := Record{Version: semver.Version{Major: uint64(i + 1)}, Class: c, PublishedAt: t, NewContentFrom: nc, EnforcedFrom: ef}
		if r.Intn(5) == 0 {
			rec.RevokedFrom = ptr(nc.Add(timex.Duration(r.Intn(60)) * timex.Day))
		}
		out = append(out, rec)
	}
	return out
}

func TestStateAtPropertiesHeadPublishedOthersDeprecated(t *testing.T) {
	r := rand.New(rand.NewSource(7))
	for iter := 0; iter < 300; iter++ {
		rs := randomRecords(r, 1+r.Intn(5))
		ts := at("2026-01-01T00:00:00Z").Add(timex.Duration(r.Intn(400)) * timex.Day)
		set := InForceSet(rs, ts, 0)
		for i, v := range set {
			want := Deprecated
			if i == 0 {
				want = Published
			}
			if got := StateAt(rs, v, ts); got != want {
				t.Fatalf("iter %d %s at %s: got %s want %s", iter, v.Version, ts, got, want)
			}
		}
		for _, v := range rs {
			s := StateAt(rs, v, ts)
			inSet := InForce(rs, v, ts, 0)
			if inSet != (s == Published || s == Deprecated) {
				t.Fatalf("iter %d: %s state %s but in-force=%v", iter, v.Version, s, inSet)
			}
		}
	}
}

func TestInForceSetGrowsWithLease(t *testing.T) {
	r := rand.New(rand.NewSource(11))
	for iter := 0; iter < 300; iter++ {
		rs := randomRecords(r, 1+r.Intn(5))
		ts := at("2026-01-01T00:00:00Z").Add(timex.Duration(r.Intn(400)) * timex.Day)
		for _, v := range InForceSet(rs, ts, 0) {
			if !InForce(rs, v, ts, 10) {
				t.Fatalf("iter %d: %s in force without lease but not with it", iter, v.Version)
			}
		}
	}
}

func TestT_HUB_007_MergeKeepsDatesAndContinuityDetectsChanges(t *testing.T) {
	prev := &Accepted{Sequence: 418, Raw: []byte("a"), Known: map[string]Record{"1.0.0": f6()[0], "2.0.0": f6()[1]}}
	next := Listing{Sequence: 419, Versions: f6()}
	next.Versions[1].EnforcedFrom = next.Versions[1].EnforcedFrom.Add(-timex.Day)
	if _, err := CheckContinuity(prev, next, []byte("b")); codeOf(err) != codes.SyncDatesChanged {
		t.Fatalf("got %v", err)
	}
}

func TestContinuityRollbackConflictAndDrop(t *testing.T) {
	revoked := f6()
	revoked[0].RevokedFrom = ptr(at("2026-11-12T00:00:00Z"))
	prev := &Accepted{Sequence: 418, Raw: []byte("a"), Known: map[string]Record{"1.0.0": revoked[0], "2.0.0": revoked[1]}}
	cases := []struct {
		next Listing
		raw  string
		want codes.Code
	}{
		{Listing{Sequence: 417, Versions: revoked}, "x", codes.SyncRollback},
		{Listing{Sequence: 418, Versions: revoked}, "x", codes.SyncSequenceConflict},
		{Listing{Sequence: 419, Versions: f6()[1:]}, "x", codes.SyncRevocationDropped},
		{Listing{Sequence: 419, Versions: revoked}, "x", ""},
	}
	for _, c := range cases {
		if _, err := CheckContinuity(prev, c.next, []byte(c.raw)); codeOf(err) != c.want {
			t.Errorf("seq %d: got %v want %s", c.next.Sequence, err, c.want)
		}
	}
}

func TestCheckRecordsS7(t *testing.T) {
	p := func(semver.Version) Params { return Defaults() }
	if err := CheckRecords(f6(), p); err != nil {
		t.Fatal(err)
	}
	bad := f6()
	bad[1].NewContentFrom = bad[1].PublishedAt.Add(timex.Day)
	if err := CheckRecords(bad, p); codeOf(err) != codes.SyncDatesInvalid {
		t.Fatalf("got %v", err)
	}
}

func TestParamsDefaultsValidate(t *testing.T) {
	if errs := Defaults().Validate(); len(errs) != 0 {
		t.Fatalf("defaults invalid: %v", errs)
	}
}

func TestClassOf(t *testing.T) {
	one := semver.MustParse("1.0.0")
	cases := map[string]Class{"1.0.1": Compatible, "1.1.0": Compatible, "2.0.0": Breaking}
	for v, want := range cases {
		if got, ok := ClassOf(semver.MustParse(v), &one); !ok || got != want {
			t.Errorf("%s: got %s", v, got)
		}
	}
	if _, ok := ClassOf(one, &one); ok {
		t.Fatal("non-increasing version accepted")
	}
}
