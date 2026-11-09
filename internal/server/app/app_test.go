// Tests of the alexandria-server use cases against the in-memory doubles of testkit.
// Realise T-HUB-001..004, 007, 008, 010, 014, 015, 020 and 021 at application level (F8 §8.7).
package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
	"github.com/ging/alexandria/internal/testkit"
)

// codeOf extracts the first code of an application error.
func codeOf(err error) codes.Code {
	var f app.Findings
	if errors.As(err, &f) {
		return f[0].Code
	}
	if ce, ok := codes.As(err); ok {
		return ce.Code
	}
	return ""
}

// listing decodes the signed part of a raw listing envelope.
func listing(t *testing.T, raw []byte) lifecycle.Listing {
	t.Helper()
	var env sign.Envelope[lifecycle.Listing]
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	return env.Signed
}

// latest returns the current listing seen by a member.
func latest(t *testing.T, s *testkit.Server) lifecycle.Listing {
	t.Helper()
	raw, err := s.Hub.Listing.Latest(testkit.Member)
	if err != nil {
		t.Fatal(err)
	}
	return listing(t, raw)
}

// nothingVisible asserts that version v is neither in the listing nor in /pub.
func nothingVisible(t *testing.T, s *testkit.Server, v string) {
	t.Helper()
	if _, _, err := s.Hub.Publication.VersionedFile("demo", v, "manifest.json"); codeOf(err) != codes.APINotFound {
		t.Errorf("/pub serves %s: %v", v, err)
	}
	for _, l := range (testkit.MemListings{MemDB: s.DB}).All() {
		for _, r := range l.Listing.Versions {
			if r.Version.String() == v {
				t.Errorf("listing %d contains %s", l.Sequence, v)
			}
		}
	}
}

func TestT_HUB_001_NegativeDraftCorpus(t *testing.T) {
	corpus := map[codes.Code][]byte{
		codes.LifecycleGraceZero:      testkit.DraftWith(testkit.DraftV1, `{"grace_window":"P0D"}`),
		codes.LifecycleSyncInterval:   testkit.DraftWith(testkit.DraftV1, `{"freshness":{"ttl":"PT24H"},"sync_interval":"PT13H"}`),
		codes.LifecycleClockSkew:      testkit.DraftWith(testkit.DraftV1, `{"clock_skew":"PT0S"}`),
		codes.LifecycleLease:          testkit.DraftWith(testkit.DraftV1, `{"clock_skew":"PT10S","lease_margin":"PT5S"}`),
		codes.LifecyclePropagation:    testkit.DraftWith(testkit.DraftV1, `{"freshness":{"ttl":"PT24H"},"propagation_window":"PT24H"}`),
		codes.LifecycleMaxVersions:    testkit.DraftWith(testkit.DraftV1, `{"max_concurrent_versions":1}`),
		codes.LifecycleStalenessLtTTL: testkit.DraftWith(testkit.DraftV1, `{"freshness":{"ttl":"PT24H","max_staleness":"PT12H"},"sync_interval":"PT6H"}`),
		compiler_poc.PocN1:            []byte(`{`),
		compiler_poc.PocN2:            []byte(`{"space":"x","version":"1.0","slots":[{"slot":"odrl","obligation":"optional"}],"files":[]}`),
		compiler_poc.PocN3:            bytes.Replace(testkit.DraftV1, []byte(`"type":"object"`), []byte(`"type":"objekt"`), 1),
	}
	for want, doc := range corpus {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		_, err := s.Hub.Authoring.CreateDraft(testkit.Editor, doc)
		var f app.Findings
		if !errors.As(err, &f) || len(f) != 1 || f[0].Code != want {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestT_HUB_001_N4AndN5OnPropose(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Editor, testkit.Draft("0.9.0", "odrl:use"))
	if _, err := s.Hub.Authoring.Propose(testkit.Editor, d.ID); codeOf(err) != compiler_poc.PocN4 {
		t.Fatalf("N4: got %v", err)
	}
	bad := bytes.Replace(testkit.Draft("1.1.0", "odrl:use"), []byte(`"slot":"odrl","applies_to"`), []byte(`"slot":"dcat","applies_to"`), 1)
	d, _ = s.Hub.Authoring.CreateDraft(testkit.Editor, bad)
	if _, err := s.Hub.Authoring.Propose(testkit.Editor, d.ID); codeOf(err) != compiler_poc.PocN5 {
		t.Fatalf("N5: got %v", err)
	}
}

func TestT_HUB_002_CandidateImmutableAfterDraftEdit(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Editor, testkit.DraftV1)
	c1, err := s.Hub.Authoring.Propose(testkit.Editor, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Authoring.ReplaceDraft(testkit.Editor, d.ID, d.ETag, testkit.Draft("1.0.0", "odrl:use")); err != nil {
		t.Fatal(err)
	}
	c2, _ := s.Hub.Query.Candidate(testkit.Releaser, c1.ID)
	a, _ := json.Marshal(c1)
	b, _ := json.Marshal(c2)
	if !bytes.Equal(a, b) || !bytes.Equal(c1.Package.Manifest, c2.Package.Manifest) {
		t.Fatalf("candidate changed:\n%s\n%s", a, b)
	}
}

func TestT_HUB_002_ApproveTwiceInvalidState(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	id := s.Propose(t, testkit.DraftV1)
	if _, err := s.Hub.Release.Approve(testkit.Releaser, id, app.ApproveOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Release.Approve(testkit.Releaser, id, app.ApproveOptions{}); codeOf(err) != codes.APIInvalidState {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Hub.Release.Reject(testkit.Releaser, id, "late"); codeOf(err) != codes.APIInvalidState {
		t.Fatalf("reject approved: got %v", err)
	}
}

func TestT_HUB_003_EditorCannotApprove(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	id := s.Propose(t, testkit.DraftV1)
	if _, err := s.Hub.Release.Approve(testkit.Editor, id, app.ApproveOptions{}); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
	if recs, _ := (testkit.MemReleases{MemDB: s.DB}).Records(); len(recs) != 0 {
		t.Fatal("release created without releaser")
	}
	ev, _ := s.Hub.Query.Events(testkit.Releaser, 0, 100)
	for _, e := range ev {
		if e.Type == domain.EvCandidateApproved {
			t.Fatal("candidate.approved recorded")
		}
	}
}

func TestT_HUB_004_FaultInjectionPerStep(t *testing.T) {
	boom := errors.New("injected")
	steps := []struct {
		name   string
		inject func(*testkit.Server)
		opts   app.ApproveOptions
		want   codes.Code
	}{
		{"1-invariants", func(*testkit.Server) {}, app.ApproveOptions{GraceWindow: ptr(timex.Day)}, codes.DatesGraceOnCompatible},
		{"2-sign", func(s *testkit.Server) { s.Signer.Fail = boom }, app.ApproveOptions{}, codes.ReleaseAborted},
		{"3-register", func(s *testkit.Server) { s.DB.Faults.Register = boom }, app.ApproveOptions{}, codes.ReleaseAborted},
		{"4-blobs", func(s *testkit.Server) { s.DB.Faults.BlobPut = boom }, app.ApproveOptions{}, codes.ReleaseAborted},
		{"4-rows", func(s *testkit.Server) { s.DB.Faults.AppendRelease = boom }, app.ApproveOptions{}, codes.ReleaseAborted},
	}
	for _, st := range steps {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		id := s.Propose(t, testkit.DraftV1)
		st.inject(s)
		if _, err := s.Hub.Release.Approve(testkit.Releaser, id, st.opts); codeOf(err) != st.want {
			t.Errorf("%s: got %v want %s", st.name, err, st.want)
		}
		nothingVisible(t, s, "1.0.0")
		if c, _ := s.Hub.Query.Candidate(testkit.Releaser, id); c.State != domain.Open {
			t.Errorf("%s: candidate left %s", st.name, c.State)
		}
	}
}

func TestT_HUB_004_ReleaseVisibleAfterAllSteps(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	l := latest(t, s)
	if len(l.Versions) != 1 || l.IssuedAt != l.Versions[0].PublishedAt {
		t.Fatalf("listing %+v", l)
	}
	if _, _, err := s.Hub.Publication.VersionedFile("demo", "1.0.0", "manifest.json"); err != nil {
		t.Fatal(err)
	}
}

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

func TestT_HUB_005_ApproveUsesPublicationInstant(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	l := latest(t, s)
	r := l.Versions[1]
	if r.PublishedAt != timex.MustInstant("2026-10-01T10:00:00Z") || r.EnforcedFrom != timex.MustInstant("2026-11-02T10:00:00Z") || r.Class != lifecycle.Breaking {
		t.Fatalf("got %+v", r)
	}
}

func TestT_HUB_006_ApprovalRejectedOnDate1(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-10-05T10:00:00Z")
	id := s.Propose(t, testkit.Draft("3.0.0", "odrl:use"))
	if _, err := s.Hub.Release.Approve(testkit.Releaser, id, app.ApproveOptions{GraceWindow: ptr(5 * timex.Day)}); codeOf(err) != codes.DatesNonMonotonic {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_007_DatesStableAcrossListings(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "superseded"}); err != nil {
		t.Fatal(err)
	}
	all := (testkit.MemListings{MemDB: s.DB}).All()
	seen := map[string]lifecycle.Record{}
	for _, l := range all {
		for _, r := range l.Listing.Versions {
			if p, ok := seen[r.Version.String()]; ok {
				if !p.SameDates(r) || (p.RevokedFrom != nil && (r.RevokedFrom == nil || *p.RevokedFrom != *r.RevokedFrom)) {
					t.Fatalf("dates of %s changed in listing %d", r.Version, l.Sequence)
				}
			}
			seen[r.Version.String()] = r
		}
	}
	if seen["1.0.0"].RevokedFrom == nil {
		t.Fatal("revoked_from not added")
	}
}

func TestT_HUB_008_IssuesAtMostEveryHalfTTL(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	for i := 0; i < 72; i++ {
		s.Clock.Advance(timex.Hour)
		if i == 30 {
			s.Publish(t, s.Clock.Now().String(), testkit.Draft("1.1.0", "odrl:use"), app.ApproveOptions{})
		}
		if err := s.Hub.Listing.IssueIfStale(s.Clock.Now()); err != nil {
			t.Fatal(err)
		}
	}
	all := (testkit.MemListings{MemDB: s.DB}).All()
	for i, l := range all {
		if l.Listing.ExpiresAt.Sub(l.Listing.IssuedAt) != 24*timex.Hour {
			t.Errorf("listing %d: expires_at − issued_at = %s", l.Sequence, l.Listing.ExpiresAt.Sub(l.Listing.IssuedAt))
		}
		if i == 0 {
			continue
		}
		if l.Sequence != all[i-1].Sequence+1 || l.Listing.IssuedAt.Sub(all[i-1].Listing.IssuedAt) > 12*timex.Hour {
			t.Errorf("listing %d breaks cadence after %d", l.Sequence, all[i-1].Sequence)
		}
	}
	if len(all) < 7 {
		t.Fatalf("only %d listings in 3·ttl", len(all))
	}
}

func TestT_HUB_010_IndependentJCSAndSignature(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	raw, _ := s.Hub.Listing.Latest(testkit.Member)
	env, err := sign.ParseRaw(raw)
	if err != nil {
		t.Fatal(err)
	}
	fromBytes, _ := env.Input()
	fromStruct, _ := canon.Marshal(listing(t, raw))
	if !bytes.Equal(fromBytes, fromStruct) {
		t.Fatalf("two JCS serializations differ:\n%s\n%s", fromBytes, fromStruct)
	}
	key, ok := s.Hub.Listing.DIDDocument().AssertionKey(env.Signature.Kid)
	if !ok || sign.Verify(fromBytes, env.Signature, key) != nil {
		t.Fatal("listing signature does not verify against the DID document")
	}
}

func TestT_HUB_014_RevocationIssuesListing(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	before := latest(t, s).Sequence
	if _, err := s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"}); err != nil {
		t.Fatal(err)
	}
	l := latest(t, s)
	if l.Sequence != before+1 || len(l.Versions) != 2 || l.Versions[0].RevokedFrom == nil || *l.Versions[0].RevokedFrom != timex.MustInstant("2026-11-12T00:00:00Z") {
		t.Fatalf("got %+v", l)
	}
	if _, err := s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"}); codeOf(err) != codes.DatesAlreadyRevoked {
		t.Fatalf("second revocation: %v", err)
	}
}

func TestT_HUB_015_RotationPublishesBeforeSigning(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	kid := s.Signer.AddUnpublished()
	if err := s.Signer.Activate(kid); err == nil {
		t.Fatal("unpublished key activated")
	}
	s.Signer.Publish(kid)
	if err := s.Signer.Activate(kid); err != nil {
		t.Fatal(err)
	}
	s.Clock.Advance(13 * timex.Hour)
	raw, _ := s.Hub.Listing.Latest(testkit.Member)
	env, _ := sign.ParseRaw(raw)
	if env.Signature.Kid != kid || !s.Hub.Listing.DIDDocument().HasKid(kid) {
		t.Fatalf("listing signed with %s", env.Signature.Kid)
	}
}

func TestT_HUB_020_EventsForEveryEffect(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Editor, testkit.DraftV1)
	d, _ = s.Hub.Authoring.ReplaceDraft(testkit.Editor, d.ID, d.ETag, testkit.DraftV1)
	c, _ := s.Hub.Authoring.Propose(testkit.Editor, d.ID)
	_, _ = s.Hub.Release.Reject(testkit.Releaser, c.ID, "no")
	c, _ = s.Hub.Authoring.Propose(testkit.Editor, d.ID)
	_, _ = s.Hub.Release.Approve(testkit.Releaser, c.ID, app.ApproveOptions{})
	ev, _ := s.Hub.Query.Events(testkit.Editor, 0, 100)
	want := []string{
		domain.EvDraftCreated, domain.EvDraftUpdated, domain.EvCandidateProposed, domain.EvCandidateRejected,
		domain.EvCandidateProposed, domain.EvCandidateApproved, domain.EvReleasePublished, domain.EvListingIssued,
	}
	if len(ev) != len(want) {
		t.Fatalf("got %d events: %+v", len(ev), ev)
	}
	for i, e := range ev {
		if e.Type != want[i] || (i > 0 && e.ID <= ev[i-1].ID) {
			t.Errorf("event %d: %+v", i, e)
		}
	}
}

func TestT_HUB_021_RecordEqualsListingEntry(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	for _, r := range latest(t, s).Versions {
		raw, err := s.Hub.Listing.Record(testkit.Member, r.Version.String())
		if err != nil {
			t.Fatal(err)
		}
		var env sign.Envelope[lifecycle.VersionDoc]
		_ = json.Unmarshal(raw, &env)
		a, _ := json.Marshal(env.Signed.Record)
		b, _ := json.Marshal(r)
		if !bytes.Equal(a, b) {
			t.Errorf("%s: record %s differs from listing %s", r.Version, a, b)
		}
	}
}
