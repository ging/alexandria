// Tests drafts, candidates, release order with fault injection, dates, revocation, listings and the signing-key gate.
// Fixtures: the F6 drafts of testkit, published on 2026-09-01 and 2026-10-01 (T-HUB-001…015, 018, 020, 021, 031).
package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
	"github.com/ging/alexandria/internal/testkit"
)

// ptr returns a pointer to v.
func ptr[T any](v T) *T { return &v }

// lifecycleCases maps each LIFECYCLE_* code to a lifecycle object that triggers it alone (F5 negative cases).
var lifecycleCases = map[codes.Code]string{
	codes.LifecycleStalenessLtTTL: `{"freshness":{"ttl":"PT24H","max_staleness":"PT12H"}}`,
	codes.LifecycleMinGrace:       `{"min_grace_window":"P1D"}`,
	codes.LifecycleGraceTooShort:  `{"grace_window":"P5D"}`,
	codes.LifecycleSyncInterval:   `{"sync_interval":"PT13H"}`,
	codes.LifecycleClockSkew:      `{"clock_skew":"PT0S"}`,
	codes.LifecycleLease:          `{"clock_skew":"PT10S","lease_margin":"PT5S"}`,
	codes.LifecyclePropagation:    `{"propagation_window":"PT24H","min_grace_window":"PT24H"}`,
	codes.LifecycleMaxVersions:    `{"max_concurrent_versions":1}`,
}

func TestT_HUB_001_LifecycleCorpus(t *testing.T) {
	s := newHub(t)
	for want, lc := range lifecycleCases {
		_, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftWith(testkit.DraftV1, lc))
		if codeOf(err) != want {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestT_HUB_001_PocN1ToN3(t *testing.T) {
	s := newHub(t)
	bad := map[codes.Code][]byte{
		"POC_N1": []byte("{"),
		"POC_N2": []byte(`{"version":"1.0.0","slots":[],"files":[],"extra":1}`),
		"POC_N3": []byte(`{"version":"1.0.0","slots":[{"slot":"p","obligation":"mandatory"}],"files":[{"path":"s.json","media_type":"application/schema+json","role":"json-schema","slot":"p","content":{"type":"nope"}}]}`),
	}
	for want, doc := range bad {
		if _, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, doc); codeOf(err) != want {
			t.Errorf("%s: got %v", want, err)
		}
	}
}

func TestT_HUB_001_PocN4OnPropose(t *testing.T) {
	s := newHub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
	if _, err := s.Hub.Authoring.Propose(testkit.Sergio, testkit.NS, d.ID); codeOf(err) != "POC_N4" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_001_PocN5OnPropose(t *testing.T) {
	s := newHub(t)
	doc := []byte(`{"version":"1.0.0","slots":[{"slot":"p","obligation":"mandatory"}],"files":[{"path":"a.json","media_type":"application/json","role":"other","slot":"q","content":{}}]}`)
	d, err := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, doc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Authoring.Propose(testkit.Sergio, testkit.NS, d.ID); codeOf(err) != "POC_N5" {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_002_CandidateFrozenAfterDraftEdit(t *testing.T) {
	s := newHub(t)
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
	c, _ := s.Hub.Authoring.Propose(testkit.Sergio, testkit.NS, d.ID)
	if _, err := s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, d.ETag, testkit.Draft("1.0.0", "odrl:use")); err != nil {
		t.Fatal(err)
	}
	again, _ := s.Hub.Query.Candidate(testkit.Sergio, testkit.NS, c.ID)
	a, _ := json.Marshal(c.Package)
	b, _ := json.Marshal(again.Package)
	if !bytes.Equal(a, b) {
		t.Fatal("candidate artefacts changed after a draft edit")
	}
}

func TestT_HUB_002_ApproveTwiceInvalidState(t *testing.T) {
	s := newHub(t)
	s.Clock.Set("2026-09-01T10:00:00Z")
	id := propose(t, s, testkit.DraftV1)
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{}); codeOf(err) != codes.APIInvalidState {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_003_NonStewardCannotApprove(t *testing.T) {
	s := newHub(t)
	id := propose(t, s, testkit.DraftV1)
	if _, err := s.Hub.Release.Approve(testkit.Eve, testkit.NS, id, app.ApproveOptions{}); codeOf(err) != codes.APIForbiddenRole {
		t.Fatalf("got %v", err)
	}
	if _, err := s.Hub.Listing.Latest(testkit.NS); codeOf(err) != codes.APINotFound {
		t.Fatalf("a release exists: %v", err)
	}
}

func TestT_HUB_004_BlobFailureAborts(t *testing.T) {
	s := newHub(t)
	id := propose(t, s, testkit.DraftV1)
	s.DB.Faults.BlobPut = errors.New("disk full")
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{}); codeOf(err) != codes.ReleaseAborted {
		t.Fatalf("got %v", err)
	}
	if len(s.DB.Listings(testkit.NS)) != 0 {
		t.Fatal("a listing was issued after an aborted release")
	}
}

func TestT_HUB_004_RowFailureLeavesCandidateOpen(t *testing.T) {
	s := newHub(t)
	id := propose(t, s, testkit.DraftV1)
	s.DB.Faults.AppendRelease = errors.New("db down")
	_, _ = s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
	c, _ := s.Hub.Query.Candidate(testkit.Sergio, testkit.NS, id)
	if c.State != domain.Open {
		t.Fatalf("candidate is %s", c.State)
	}
}

func TestT_HUB_004_ListingIssuedAtP(t *testing.T) {
	s := newHub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	l := listing(t, s)
	if l.ValidFrom != l.Subject.Versions[0].PublishedAt {
		t.Fatalf("issued_at %s, published_at %s", l.ValidFrom, l.Subject.Versions[0].PublishedAt)
	}
}

func TestT_HUB_005_F6Dates(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	v := listing(t, s).Subject.Versions
	if v[1].NewContentFrom != timex.MustInstant("2026-10-03T10:00:00Z") || v[1].EnforcedFrom != timex.MustInstant("2026-11-02T10:00:00Z") || v[1].Class != lifecycle.Breaking {
		t.Fatalf("got %+v", v[1])
	}
}

func TestT_HUB_005_GraceTooShortPublishesNothing(t *testing.T) {
	s := newHub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	s.Clock.Set("2026-10-01T10:00:00Z")
	id := propose(t, s, testkit.DraftV2)
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{GraceWindow: ptr(3 * timex.Day)}); codeOf(err) != codes.DatesGraceTooShort {
		t.Fatalf("got %v", err)
	}
	if len(listing(t, s).Subject.Versions) != 1 {
		t.Fatal("2.0.0 was published")
	}
}

func TestT_HUB_006_Date1OnApprove(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	s.Clock.Set("2026-10-05T10:00:00Z")
	id := propose(t, s, testkit.Draft("3.0.0", "odrl:use"))
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{GraceWindow: ptr(7 * timex.Day)}); codeOf(err) != codes.DatesNonMonotonic {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_007_DatesStableAcrossListings(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	before := listing(t, s).Subject.Versions[1]
	s.Clock.Set("2026-10-10T12:00:00Z")
	after := listing(t, s).Subject.Versions[1]
	if !before.SameDates(after) {
		t.Fatal("dates changed between listings")
	}
}

func TestT_HUB_008_SequenceAndValidity(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	ls := s.DB.Listings(testkit.NS)
	for i, l := range ls {
		if l.Sequence != uint64(i+1) || l.ValidUntil.Sub(l.IssuedAt) != 24*timex.Hour {
			t.Fatalf("listing %d: seq %d validity %s", i, l.Sequence, l.ValidUntil.Sub(l.IssuedAt))
		}
	}
}

func TestT_HUB_008_ReissuedAtHalfTTL(t *testing.T) {
	s := newHub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	s.Clock.Set("2026-09-01T21:59:59Z")
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	s.Clock.Set("2026-09-01T22:00:00Z")
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	if n := len(s.DB.Listings(testkit.NS)); n != 2 {
		t.Fatalf("%d listings, want 2", n)
	}
}

func TestT_HUB_009_RetiredLeavesAndRevokedReturns(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	s.Clock.Set("2026-11-05T12:00:00Z")
	if v := versionsOf(listing(t, s).Subject); len(v) != 1 || v[0] != "2.0.0" {
		t.Fatalf("after retirement: %v", v)
	}
	s.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "unsafe"}); err != nil {
		t.Fatal(err)
	}
	if v := versionsOf(listing(t, s).Subject); len(v) != 2 {
		t.Fatalf("after revocation: %v", v)
	}
}

func TestT_HUB_010_ProofsVerifyWithDIDKey(t *testing.T) {
	s := newHub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	p, _ := vc.ParseProof(raw)
	doc, _ := did.ResolveKey(did.OfURL(p.VerificationMethod))
	k, ok := doc.AssertionKey(p.VerificationMethod)
	if !ok || vc.Verify(raw, k.Key) != nil {
		t.Fatal("listing proof does not verify with the did:key document")
	}
}

func TestT_HUB_014_RevocationInvariants(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	early := timex.MustInstant("2026-11-11T00:00:00Z")
	cases := []struct {
		v    string
		o    app.RevokeOptions
		want codes.Code
	}{
		{"1.0.0", app.RevokeOptions{}, codes.DatesJustificationRequired},
		{"1.0.0", app.RevokeOptions{Justification: "x", RevokedFrom: &early}, codes.DatesRevocationTooEarly},
		{"2.0.0", app.RevokeOptions{Justification: "x"}, codes.DatesRevokeLeavesNoHead},
	}
	for _, c := range cases {
		if _, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, c.v, c.o); codeOf(err) != c.want {
			t.Errorf("%s: got %v", c.want, err)
		}
	}
}

func TestT_HUB_014_AlreadyRevoked(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
	if _, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"}); codeOf(err) != codes.DatesAlreadyRevoked {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_015_UnpublishedKeyNeverSigns(t *testing.T) {
	s := newHub(t)
	s.Signer.Rotate(9, "did:key:z6MkOther#z6MkOther")
	if s.Hub.Keys.CheckSigningKey() {
		t.Fatal("unpublished key accepted")
	}
	s.Clock.Set("2026-09-01T10:00:00Z")
	id := propose(t, s, testkit.DraftV1)
	if _, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{}); codeOf(err) != codes.SignerKeyNotPublished {
		t.Fatalf("got %v", err)
	}
	ev, _ := s.Hub.Query.Events(testkit.Ana, "", 0, 100)
	if ev[len(ev)-1].Type != domain.EvSignerKeyUnpublished {
		t.Fatal("signer.key_unpublished not emitted")
	}
}

func TestT_HUB_018_PreconditionRequired(t *testing.T) {
	s := newHub(t)
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
	if _, err := s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, "", testkit.DraftV1); codeOf(err) != codes.APIPreconditionRequired {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_018_PreconditionFailed(t *testing.T) {
	s := newHub(t)
	d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
	_, _ = s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, d.ETag, testkit.DraftV1)
	if _, err := s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, d.ETag, testkit.DraftV1); codeOf(err) != codes.APIPreconditionFailed {
		t.Fatalf("got %v", err)
	}
}

func TestT_HUB_020_EventIDsStrictlyIncrease(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	ev, _ := s.Hub.Query.Events(testkit.Sergio, testkit.NS, 0, 500)
	for i := 1; i < len(ev); i++ {
		if ev[i].ID <= ev[i-1].ID {
			t.Fatalf("event %d does not increase", i)
		}
	}
	if len(ev) < 8 {
		t.Fatalf("only %d namespace events", len(ev))
	}
}

func TestT_HUB_021_RecordMatchesListingEntry(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	for _, entry := range listing(t, s).Subject.Versions {
		raw, _ := s.Hub.Listing.Record(testkit.NS, entry.Version.String())
		var c vc.Credential[lifecycle.Record]
		_ = json.Unmarshal(raw, &c)
		a, _ := json.Marshal(c.Subject)
		b, _ := json.Marshal(entry)
		if !bytes.Equal(a, b) {
			t.Fatalf("record %s differs from listing entry", entry.Version)
		}
	}
}

func TestT_HUB_031_CredentialsValidateAgainstSchemas(t *testing.T) {
	s := newHub(t)
	s.PublishF6(t)
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	if err := schemas.Validate(schemas.Versions, raw); err != nil {
		t.Errorf("listing: %v", err)
	}
	rec, _ := s.Hub.Listing.Record(testkit.NS, "2.0.0")
	if err := schemas.Validate(schemas.Version, rec); err != nil {
		t.Errorf("record: %v", err)
	}
	var c vc.Credential[lifecycle.Record]
	_ = json.Unmarshal(rec, &c)
	m, _ := s.Hub.Publication.Manifest(c.Subject.Manifest.Segment())
	if err := schemas.Validate(schemas.Manifest, m); err != nil {
		t.Errorf("manifest: %v", err)
	}
}
