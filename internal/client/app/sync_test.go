// Tests Sync S1–S13 with one negative listing or package per step, persistence across restarts and the V0–V4 trust procedure.
// Fixtures: the testkit hub publishing F6 and listings re-signed with the hub key after one mutation (T-CLI-003..007, 024, 027, 034).
package app_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// f6 returns a hub with both F6 releases and a synced client at 2026-10-10T12:00:00Z.
func f6(t *testing.T) (*testkit.Server, *testkit.Client) {
	t.Helper()
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-10-10T12:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	return s, c
}

// failsWith runs a sync and checks its code.
func failsWith(t *testing.T, c *testkit.Client, want codes.Code) {
	t.Helper()
	if err := c.Sync.Run(); err == nil || err.Code != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}

// forge serves the current listing re-signed after mutate.
func forge(t *testing.T, s *testkit.Server, c *testkit.Client, mutate func(map[string]any)) {
	t.Helper()
	c.Hub.ListingHook = func(b []byte) []byte { return s.Resign(t, b, mutate) }
}

// versions returns the versions array of a decoded listing.
func versions(doc map[string]any) []any { return testkit.Subject(doc)["versions"].([]any) }

func TestT_CLI_004_S1Unreachable(t *testing.T) {
	_, c := f6(t)
	c.Hub.Down = true
	failsWith(t, c, codes.SyncUnreachable)
}

func TestT_CLI_004_S1NamespaceNotFound(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	failsWith(t, testkit.NewClient(t, s), codes.SyncNamespaceNotFound)
}

func TestT_CLI_034_S2IssuerMismatch(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { d["issuer"] = "did:key:z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH" })
	failsWith(t, c, codes.SyncMalformed)
}

func TestT_CLI_034_S2ContextMismatch(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { d["@context"] = []any{"https://www.w3.org/ns/credentials/v2", "https://w3id.org/alexandria-vhub/v1"} })
	failsWith(t, c, codes.SyncMalformed)
}

func TestT_CLI_034_S2NamespaceMismatch(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["namespace"] = testkit.Base + "/v1/namespaces/other" })
	failsWith(t, c, codes.SyncMalformed)
}

func TestT_CLI_034_S2SchemaViolation(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { d["extra"] = 1 })
	failsWith(t, c, codes.SyncMalformed)
}

func TestT_CLI_024_S2AlteredByte(t *testing.T) {
	_, c := f6(t)
	c.Hub.ListingHook = func(b []byte) []byte { return bytes.Replace(b, []byte(`"sequence":`), []byte(`"sequence":1`), 1) }
	failsWith(t, c, codes.SyncSignatureInvalid)
}

func TestT_CLI_004_S3Expired(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { d["validUntil"] = "2026-10-10T11:00:00Z" })
	failsWith(t, c, codes.SyncExpired)
}

func TestT_CLI_004_S4Rollback(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["sequence"] = float64(1) })
	failsWith(t, c, codes.SyncRollback)
}

func TestT_CLI_004_S4SequenceConflict(t *testing.T) {
	s, c := f6(t)
	forge(t, s, c, func(d map[string]any) { d["validUntil"] = "2026-10-11T13:00:00Z" })
	failsWith(t, c, codes.SyncSequenceConflict)
}

func TestT_CLI_004_S5RevocationDropped(t *testing.T) {
	s, c := f6(t)
	s.Clock.Set("2026-11-10T00:00:00Z")
	_, _ = s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", revoke)
	c.SyncNow(t)
	s.Clock.Set("2026-11-10T12:00:00Z")
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["versions"] = versions(d)[1:] })
	failsWith(t, c, codes.SyncRevocationDropped)
}

func TestT_CLI_004_S6DatesChanged(t *testing.T) {
	s, c := f6(t)
	s.Clock.Set("2026-10-11T00:00:00Z")
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	forge(t, s, c, func(d map[string]any) { versions(d)[1].(map[string]any)["enforced_from"] = "2026-11-01T10:00:00Z" })
	failsWith(t, c, codes.SyncDatesChanged)
}

func TestT_CLI_006_S7BreakingWithoutMinGrace(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-10-10T12:00:00Z")
	c := testkit.NewClient(t, s)
	forge(t, s, c, func(d map[string]any) { versions(d)[1].(map[string]any)["enforced_from"] = "2026-10-06T10:00:00Z" })
	failsWith(t, c, codes.SyncDatesInvalid)
	if c.Cell.Load() != nil {
		t.Fatal("a listing with an invalid version was partly accepted")
	}
}

func TestT_CLI_006_S8ManifestDigestMismatch(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	c.Hub.ManifestHook = func(_ ni.Digest, m []byte) []byte { return append(m, ' ') }
	failsWith(t, c, codes.SyncManifestDigestMismatch)
}

func TestT_CLI_006_S9ManifestAlteredAndRedigested(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	c := testkit.NewClient(t, s)
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	orig := ni.Digest(versions(decode(t, raw))[0].(map[string]any)["manifest"].(string))
	f, err := c.Hub.Package(orig)
	if err != nil {
		t.Fatal(err)
	}
	f.Manifest = bytes.Replace(f.Manifest, []byte(`"role":"other"`), []byte(`"role":"shacl"`), 1)
	bad := ni.Sum(f.Manifest)
	c.Hub.PackageHook = func(d ni.Digest) (pkgv.Files, bool) { return f, d == bad }
	forge(t, s, c, func(d map[string]any) { versions(d)[0].(map[string]any)["manifest"] = string(bad) })
	failsWith(t, c, codes.SyncManifestSignatureInvalid)
}

func TestT_CLI_007_S11FileHashMismatch(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	c.Hub.FilesHook = func(f map[string][]byte) { f["schemas/policy.schema.json"] = append([]byte(" "), f["schemas/policy.schema.json"]...) }
	failsWith(t, c, codes.SyncFileHashMismatch)
}

func TestT_CLI_007_S12ExtraFile(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	c.Hub.FilesHook = func(f map[string][]byte) { f["extra.json"] = []byte(`{}`) }
	failsWith(t, c, codes.SyncFileSetMismatch)
}

func TestT_CLI_007_S13VersionIncoherent(t *testing.T) {
	s := testkit.NewServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	c := testkit.NewClient(t, s)
	forge(t, s, c, func(d map[string]any) {
		v := versions(d)[1].(map[string]any)
		v["version"], v["id"] = "2.0.1", testkit.NSIRI+"/versions/2.0.1"
	})
	failsWith(t, c, codes.SyncIncoherent)
}

func TestT_CLI_003_SyncsAtStartAndAfterInterval(t *testing.T) {
	s, c := f6(t)
	if c.Sync.Due(s.Clock.Now()) {
		t.Fatal("due right after a sync")
	}
	s.Clock.Advance(6 * 3600)
	if !c.Sync.Due(s.Clock.Now()) {
		t.Fatal("not due after sync_interval")
	}
}

func TestT_CLI_005_ContinuitySurvivesRestart(t *testing.T) {
	s, c := f6(t)
	c.Restart(t, testkit.SeedPath(t))
	if c.Cell.Load() == nil {
		t.Fatal("snapshot not restored")
	}
	forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["sequence"] = float64(1) })
	failsWith(t, c, codes.SyncRollback)
}

func TestT_CLI_024_RotationResolvesOnceAndEmits(t *testing.T) {
	s, c := webClient(t)
	s.Signer.Rotate(7, string(testkit.WebDID)+"#key-2")
	s.Resolver.SetDocument(testkit.WebDID, did.PublicKey{Kid: string(testkit.WebDID) + "#key-2", Key: s.Signer.PublicKey()})
	s.Hub.Keys.CheckSigningKey()
	s.Clock.Advance(120)
	if _, err := s.Hub.Listing.Issue(testkit.NS, s.Clock.Now()); err != nil {
		t.Fatal(err)
	}
	c.SyncNow(t)
	if !c.Events.Has(string(codes.DidKeyRotated)) {
		t.Fatal("DID_KEY_ROTATED not emitted")
	}
}

func TestT_CLI_024_RetiredKeyNoLongerAccepted(t *testing.T) {
	s, c := webClient(t)
	s.Resolver.SetDocument(testkit.WebDID)
	s.Clock.Advance(7 * 3600)
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	failsWith(t, c, codes.SyncSignatureInvalid)
}

func TestT_CLI_027_HundredBadProofsOneResolution(t *testing.T) {
	_, c := webClient(t)
	c.Hub.ListingHook = func(b []byte) []byte { return bytes.Replace(b, []byte(`"sequence":`), []byte(`"sequence":9`), 1) }
	before := c.Trust.Resolutions
	for i := 0; i < 100; i++ {
		_ = c.Sync.Run()
	}
	if n := c.Trust.Resolutions - before; n > 1 {
		t.Fatalf("%d resolutions in one minute", n)
	}
}

func TestT_CLI_027_DIDUnresolvableOnFirstStart(t *testing.T) {
	s := testkit.NewWebServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	s.Resolver.Fail = true
	failsWith(t, testkit.NewClient(t, s), codes.SyncDidUnresolvable)
}

func TestT_CLI_027_RefreshFailedKeepsCache(t *testing.T) {
	s, c := webClient(t)
	s.Resolver.Fail = true
	s.Clock.Advance(7 * 3600)
	_ = s.Hub.Listing.Tick(s.Clock.Now())
	c.SyncNow(t)
	if !c.Events.Has(string(codes.DidRefreshFailed)) {
		t.Fatal("DID_REFRESH_FAILED not emitted")
	}
}

// decode parses a JSON document into a map.
func decode(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

// revoke is a valid revocation body.
var revoke = app.RevokeOptions{Justification: "unsafe"}

// webClient returns a did:web hub with F6 and a synced client at 2026-10-10T12:00:00Z.
func webClient(t *testing.T) (*testkit.Server, *testkit.Client) {
	t.Helper()
	s := testkit.NewWebServer(t, "2026-08-01T00:00:00Z")
	s.PublishF6(t)
	s.Clock.Set("2026-10-10T12:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	return s, c
}
