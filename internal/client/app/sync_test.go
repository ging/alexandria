// Tests of synchronization S1–S13, continuity persistence and hub signature verification V1–V4.
// Realise T-CLI-003..007, T-CLI-024, T-CLI-025, T-CLI-027 and T-SYS-009 with one negative document per step (§4.4).
package app_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// codeOf returns the code of a sync result.
func codeOf(e *codes.Error) codes.Code {
	if e == nil {
		return ""
	}
	return e.Code
}

// synced returns an F6 hub and a client synced at ts.
func synced(t *testing.T, ts string) (*testkit.Server, *testkit.Client) {
	t.Helper()
	s, c := f6Client(t)
	at(t, s, c, ts)
	return s, c
}

// manifestOf returns the manifest digest of a version in the hub listing.
func manifestOf(t *testing.T, s *testkit.Server, v string) ni.Digest {
	t.Helper()
	_, l := s.LatestListing(t)
	for _, r := range l.Versions {
		if r.Version.String() == v {
			return r.Manifest
		}
	}
	t.Fatalf("%s not listed", v)
	return ""
}

// expectSync runs a sync and checks its code.
func expectSync(t *testing.T, c *testkit.Client, want codes.Code) {
	t.Helper()
	if got := codeOf(c.Sync.Run()); got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}

func TestT_CLI_003_SyncAtStartAndEveryInterval(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	if !c.Sync.Due(s.Clock.Now()) {
		t.Fatal("not due at start")
	}
	_ = c.Sync.Run()
	if c.Sync.Due(s.Clock.Now().Add(6*timex.Hour - 1)) {
		t.Fatal("due before sync_interval")
	}
	if !c.Sync.Due(s.Clock.Now().Add(6 * timex.Hour)) {
		t.Fatal("not due at sync_interval")
	}
	c.Hub.Down = true
	s.Clock.Advance(6 * timex.Hour)
	_ = c.Sync.Run()
	if !c.Sync.Due(s.Clock.Now().Add(timex.Minute)) {
		t.Fatal("no retry one minute after a failure")
	}
}

func TestT_CLI_004_S1Unreachable(t *testing.T) {
	_, c := f6Client(t)
	c.Hub.Down = true
	expectSync(t, c, codes.SyncUnreachable)
}

func TestT_CLI_004_S1Unauthorized(t *testing.T) {
	_, c := f6Client(t)
	c.Hub.Unauthorized = true
	expectSync(t, c, codes.SyncUnauthorized)
}

func TestT_CLI_004_S2AlteredByte(t *testing.T) {
	_, c := f6Client(t)
	c.Hub.Tamper = func(b []byte) []byte { return bytes.Replace(b, []byte("spaces/demo"), []byte("spaces/demp"), 1) }
	expectSync(t, c, codes.SyncSignatureInvalid)
}

func TestT_CLI_004_S2DIDUnresolvable(t *testing.T) {
	_, c := f6Client(t)
	c.Resolver.Down = true
	expectSync(t, c, codes.SyncDidUnresolvable)
}

func TestT_CLI_004_S3Expired(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	old, _ := s.LatestListing(t)
	s.Clock.Set("2026-10-03T00:00:01Z")
	c.Hub.Tamper = testkit.Serve(old)
	expectSync(t, c, codes.SyncExpired)
}

func TestT_CLI_004_S4Rollback(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	old, _ := s.LatestListing(t)
	at(t, s, c, "2026-10-02T13:00:00Z")
	c.Hub.Tamper = testkit.Serve(old)
	expectSync(t, c, codes.SyncRollback)
}

func TestT_CLI_004_S4SequenceConflict(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	c.Hub.Tamper = testkit.Serve(s.Forge(t, func(l *lifecycle.Listing) { l.ExpiresAt = l.ExpiresAt.Add(timex.Hour) }))
	expectSync(t, c, codes.SyncSequenceConflict)
}

func TestT_CLI_004_S5RevocationDropped(t *testing.T) {
	s, c := synced(t, "2026-11-05T00:00:00Z")
	s.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
	at(t, s, c, "2026-11-10T00:00:00Z")
	c.Hub.Tamper = testkit.Serve(s.Forge(t, func(l *lifecycle.Listing) { l.Sequence++; l.Versions = l.Versions[1:] }))
	expectSync(t, c, codes.SyncRevocationDropped)
}

func TestT_CLI_004_S6DatesChanged(t *testing.T) {
	s, c := synced(t, "2026-10-10T00:00:00Z")
	c.Hub.Tamper = testkit.Serve(s.Forge(t, func(l *lifecycle.Listing) {
		l.Sequence++
		l.Versions[1].EnforcedFrom = l.Versions[1].EnforcedFrom.Add(-timex.Day)
	}))
	expectSync(t, c, codes.SyncDatesChanged)
}

func TestT_CLI_004_S7DatesInvalid(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-10T00:00:00Z")
	c.Hub.Tamper = testkit.Serve(s.Forge(t, func(l *lifecycle.Listing) {
		l.Versions[1].PublishedAt = l.Versions[1].NewContentFrom.Add(-timex.Day)
	}))
	expectSync(t, c, codes.SyncDatesInvalid)
}

func TestT_CLI_005_ContinuitySurvivesRestart(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	old, _ := s.LatestListing(t)
	at(t, s, c, "2026-10-02T13:00:00Z")
	resolutions := c.Resolver.Calls
	if err := c.Configure(testkit.F6Routes(), "forward", "forward"); err != nil {
		t.Fatal(err)
	}
	if c.Cell.Load() == nil || domain.StateAt(c.Cell.Load(), s.Clock.Now()) != domain.Active {
		t.Fatal("snapshot not restored")
	}
	c.Hub.Tamper = testkit.Serve(old)
	expectSync(t, c, codes.SyncRollback)
	if c.Resolver.Calls != resolutions {
		t.Fatal("cached DID document was not persisted")
	}
}

func TestT_CLI_005_KnownRevocationSurvivesRestart(t *testing.T) {
	s, c := synced(t, "2026-11-05T00:00:00Z")
	s.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
	at(t, s, c, "2026-11-10T00:00:00Z")
	_ = c.Configure(testkit.F6Routes(), "forward", "forward")
	if r, ok := c.Cell.Load().KnownRecord("1.0.0"); !ok || r.RevokedFrom == nil {
		t.Fatal("revoked_from not persisted")
	}
}

func TestT_CLI_006_OneCorruptVersionDiscardsListing(t *testing.T) {
	s, c := f6Client(t)
	v2 := manifestOf(t, s, "2.0.0")
	c.Hub.TamperPkg = func(d ni.Digest, f *pkgv.Files) {
		if d == v2 {
			f.Manifest = append(f.Manifest, ' ')
		}
	}
	s.Clock.Set("2026-10-10T00:00:00Z")
	expectSync(t, c, codes.SyncManifestDigestMismatch)
	if c.Cell.Load() != nil {
		t.Fatal("a partial listing was accepted")
	}
}

func TestT_CLI_006_S9ManifestSignatureOfOtherManifest(t *testing.T) {
	s, c := f6Client(t)
	v1, v2 := manifestOf(t, s, "1.0.0"), manifestOf(t, s, "2.0.0")
	sig1, _ := s.Hub.Publication.PackageFile(v1.Segment(), "manifest.sig")
	c.Hub.TamperPkg = func(d ni.Digest, f *pkgv.Files) {
		if d == v2 {
			f.Sig = sig1
		}
	}
	s.Clock.Set("2026-10-10T00:00:00Z")
	expectSync(t, c, codes.SyncManifestSignatureInvalid)
}

func TestT_CLI_006_S10ReceiptOfOtherDigest(t *testing.T) {
	s, c := f6Client(t)
	v1, v2 := manifestOf(t, s, "1.0.0"), manifestOf(t, s, "2.0.0")
	rc1, _ := s.Hub.Publication.PackageFile(v1.Segment(), "manifest.receipt")
	c.Hub.TamperPkg = func(d ni.Digest, f *pkgv.Files) {
		if d == v2 {
			f.Receipt = rc1
		}
	}
	s.Clock.Set("2026-10-10T00:00:00Z")
	expectSync(t, c, codes.SyncReceiptInvalid)
}

func TestT_CLI_007_S11FileByteAltered(t *testing.T) {
	s, c := f6Client(t)
	c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) {
		for p, b := range f.Content {
			f.Content[p] = bytes.Replace(b, []byte("object"), []byte("objecT"), 1)
		}
	}
	s.Clock.Set("2026-10-10T00:00:00Z")
	expectSync(t, c, codes.SyncFileHashMismatch)
}

func TestT_CLI_007_S12ExtraFile(t *testing.T) {
	s, c := f6Client(t)
	c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) { f.Content["shapes.ttl"] = []byte("@prefix sh: <x> .") }
	s.Clock.Set("2026-10-10T00:00:00Z")
	expectSync(t, c, codes.SyncFileSetMismatch)
}

func TestT_CLI_007_S13VersionIncoherent(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-10T00:00:00Z")
	v1 := manifestOf(t, s, "1.0.0")
	c.Hub.Tamper = testkit.Serve(s.Forge(t, func(l *lifecycle.Listing) { l.Versions[1].Manifest = v1 }))
	expectSync(t, c, codes.SyncIncoherent)
}

func TestT_CLI_007_PartialDownloadNeverUsed(t *testing.T) {
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	c := testkit.NewClient(s, testkit.F6Routes())
	at(t, s, c, "2026-09-05T00:00:00Z")
	s.Publish(t, "2026-10-01T10:00:00Z", testkit.DraftV2, app.ApproveOptions{})
	s.Clock.Set("2026-10-04T12:00:00Z")
	c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) {
		for p := range f.Content {
			delete(f.Content, p)
		}
	}
	_, _ = s.LatestListing(t)
	expectSync(t, c, codes.SyncFileSetMismatch)
	if r := c.Send(domain.DSPIngress, "POST", "/negotiations/request", testkit.ContractRequest("odrl:read")); r.Status == 200 {
		t.Fatal("a 2.0.0 artefact took part in a classification before verification")
	}
}

func TestT_CLI_024_KeyRotation(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	kid := s.Signer.AddUnpublished()
	s.Signer.Publish(kid)
	_ = s.Signer.Activate(kid)
	s.Clock.Set("2026-10-02T00:05:00Z")
	_, _ = s.Hub.Listing.Issue(s.Clock.Now())
	expectSync(t, c, "")
	if !c.Events.Has("DID_KEY_ROTATED") {
		t.Fatal("DID_KEY_ROTATED not emitted")
	}
}

func TestT_CLI_024_AlteredByteResolvesAtMostOnce(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	s.Clock.Set("2026-10-02T13:00:00Z")
	before := c.Resolver.Calls
	c.Hub.Tamper = func(b []byte) []byte { return bytes.Replace(b, []byte("spaces/demo"), []byte("spaces/demp"), 1) }
	expectSync(t, c, codes.SyncSignatureInvalid)
	if c.Resolver.Calls-before != 1 {
		t.Fatalf("%d resolutions for one invalid signature", c.Resolver.Calls-before)
	}
}

func TestT_CLI_024_RetiredKeyRejected(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	old := s.Signer.PublishedKeys()[0].Kid
	kid := s.Signer.AddUnpublished()
	s.Signer.Publish(kid)
	_ = s.Signer.Activate(kid)
	s.Signer.Retire(old)
	s.Clock.Set("2026-10-02T13:00:00Z")
	_, l := s.LatestListing(t)
	l.Sequence++
	msg, _ := sign.SigningInput(l)
	sig, _ := json.Marshal(s.Signer.SignWith(old, msg))
	c.Hub.Tamper = testkit.Serve([]byte(`{"signed":` + string(msg) + `,"signature":` + string(sig) + `}`))
	expectSync(t, c, codes.SyncSignatureInvalid)
	if err := c.Configure(testkit.F6Routes(), "forward", "forward"); err != nil || c.Cell.Load() == nil {
		t.Fatal("an accepted listing must not be verified again after the key is retired")
	}
}

func TestT_CLI_025_ClockSkewExceeded(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	ahead := s.Clock.Now().Add(30)
	c.Hub.Date = &ahead
	_ = c.Sync.Run()
	if !c.Events.Has("CLOCK_SKEW_EXCEEDED") || c.Status.Report().ClockSkewEstimateMs != 30000 {
		t.Fatalf("skew not reported: %+v", c.Status.Report())
	}
}

func TestT_CLI_025_SmallSkewIsQuiet(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	ahead := s.Clock.Now().Add(3)
	c.Hub.Date = &ahead
	_ = c.Sync.Run()
	if c.Events.Has("CLOCK_SKEW_EXCEEDED") {
		t.Fatal("skew within clock_skew reported")
	}
}

func TestT_CLI_027_ResolveWhenCacheReachesDidTTL(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	before := c.Resolver.Calls
	s.Clock.Advance(6 * timex.Hour)
	expectSync(t, c, "")
	if c.Resolver.Calls != before+1 {
		t.Fatalf("got %d resolutions", c.Resolver.Calls-before)
	}
}

func TestT_CLI_027_RefreshFailedKeepsCache(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	s.Clock.Advance(6 * timex.Hour)
	c.Resolver.Down = true
	expectSync(t, c, "")
	if !c.Events.Has("DID_REFRESH_FAILED") {
		t.Fatal("DID_REFRESH_FAILED not emitted")
	}
}

func TestT_CLI_027_HundredInvalidSignaturesOneResolution(t *testing.T) {
	s, c := synced(t, "2026-10-02T00:00:00Z")
	s.Clock.Set("2026-10-02T00:05:00Z")
	before := c.Resolver.Calls
	c.Hub.Tamper = func(b []byte) []byte { return bytes.Replace(b, []byte("spaces/demo"), []byte("spaces/demp"), 1) }
	for i := 0; i < 100; i++ {
		_ = c.Sync.Run()
		if i%2 == 1 {
			s.Clock.Advance(1)
		}
	}
	if c.Resolver.Calls-before != 1 {
		t.Fatalf("%d resolutions in one minute", c.Resolver.Calls-before)
	}
}

func TestT_SYS_009_UnpublishedKeyNeverTrusted(t *testing.T) {
	s, c := f6Client(t)
	s.Clock.Set("2026-10-02T00:00:00Z")
	kid := s.Signer.AddUnpublished()
	_, l := s.LatestListing(t)
	msg, _ := sign.SigningInput(l)
	sig, _ := json.Marshal(s.Signer.SignWith(kid, msg))
	c.Hub.Tamper = testkit.Serve([]byte(`{"signed":` + string(msg) + `,"signature":` + string(sig) + `}`))
	expectSync(t, c, codes.SyncSignatureInvalid)
	if c.Cell.Load() != nil {
		t.Fatal("dates from an unverifiable listing were used")
	}
}
