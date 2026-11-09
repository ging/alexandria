// Negative battery for the client: one N-<CODE> case per code emitted by C-CLI.
// Configuration codes run the real vhub-client binary; the others replay minimal scenarios in process (F5 §5.2, check D3).
package conformance

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

func TestN_ConfigCodesStopTheBinary(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("corpus", "config", "CONFIG_*.json"))
	if len(files) != 5 {
		t.Fatalf("%d CONFIG documents, want 5", len(files))
	}
	for _, f := range files {
		code := trimExt(filepath.Base(f))
		t.Run("N-"+code, func(t *testing.T) {
			cmd := exec.Command(filepath.Join(binDir, "vhub-client"), "-config", f, "-state-dir", t.TempDir())
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			err := cmd.Run()
			ee, ok := err.(*exec.ExitError)
			if !ok || ee.ExitCode() == 0 || !strings.Contains(stderr.String(), code+":") {
				t.Fatalf("exit %v, stderr %q", err, stderr.String())
			}
			for _, other := range []string{"CONFIG_SCHEMA", "CONFIG_ROUTE_OVERLAP", "CONFIG_GATE_MESSAGE_MISMATCH", "CONFIG_CREDENTIALS_UNREADABLE", "CONFIG_UPSTREAM_MISSING"} {
				if other != code && strings.Contains(stderr.String(), other) {
					t.Fatalf("also reported %s", other)
				}
			}
		})
	}
}

// world is an F6 hub with a client on its clock.
type world struct {
	s *testkit.Server
	c *testkit.Client
}

// f6World publishes F6 and builds a client with the F6 routes.
func f6World(t *testing.T) world {
	t.Helper()
	s := testkit.NewServer("2026-09-01T10:00:00Z")
	s.PublishF6(t)
	return world{s, testkit.NewClient(s, testkit.F6Routes())}
}

// sync moves the clock and syncs, failing on error.
func (w world) sync(t *testing.T, ts string) {
	t.Helper()
	w.s.Clock.Set(ts)
	if err := w.c.Sync.Run(); err != nil {
		t.Fatalf("sync at %s: %v", ts, err)
	}
}

// syncCode moves the clock, syncs and returns the failure code.
func (w world) syncCode(ts string) string {
	w.s.Clock.Set(ts)
	if err := w.c.Sync.Run(); err != nil {
		return string(err.Code)
	}
	return ""
}

// request sends a ContractRequestMessage body and returns the error code of the answer.
func (w world) request(body []byte) string {
	return testkit.BodyCode(w.c.Send(domain.DSPIngress, "POST", "/negotiations/request", body).Body)
}

// manifest returns the manifest digest of a listed version.
func (w world) manifest(t *testing.T, v string) ni.Digest {
	t.Helper()
	_, l := w.s.LatestListing(t)
	for _, r := range l.Versions {
		if r.Version.String() == v {
			return r.Manifest
		}
	}
	return ""
}

// altered flips one byte of the listing's signed part.
func altered(b []byte) []byte {
	return bytes.Replace(b, []byte("spaces/demo"), []byte("spaces/demp"), 1)
}

// clientCase is one negative case returning the observed code and whether an event carried it.
type clientCase struct {
	code string
	run  func(t *testing.T) (string, *testkit.Recorder)
}

// syncCase builds a case whose code is the result of one sync.
func syncCase(code string, prepare func(t *testing.T, w world) string) clientCase {
	return clientCase{code, func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		return prepare(t, w), w.c.Events
	}}
}

// eventCase builds a case whose code must appear in the event stream.
func eventCase(code string, prepare func(t *testing.T, w world)) clientCase {
	return clientCase{code, func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		prepare(t, w)
		if w.c.Events.Has(code) {
			return code, w.c.Events
		}
		return "", w.c.Events
	}}
}

// clientCases lists the client negative cases of F5 (VERDICT_SHACL_WARNING is unreachable, see OPEN_QUESTIONS.md).
var clientCases = []clientCase{
	syncCase("SYNC_UNREACHABLE", func(t *testing.T, w world) string { w.c.Hub.Down = true; return w.syncCode("2026-10-02T00:00:00Z") }),
	syncCase("SYNC_UNAUTHORIZED", func(t *testing.T, w world) string {
		w.c.Hub.Unauthorized = true
		return w.syncCode("2026-10-02T00:00:00Z")
	}),
	syncCase("SYNC_SIGNATURE_INVALID", func(t *testing.T, w world) string {
		w.c.Hub.Tamper = altered
		return w.syncCode("2026-10-02T00:00:00Z")
	}),
	syncCase("SYNC_DID_UNRESOLVABLE", func(t *testing.T, w world) string {
		w.c.Resolver.Down = true
		return w.syncCode("2026-10-02T00:00:00Z")
	}),
	syncCase("SYNC_EXPIRED", func(t *testing.T, w world) string {
		w.s.Clock.Set("2026-10-02T00:00:00Z")
		old, _ := w.s.LatestListing(t)
		w.c.Hub.Tamper = testkit.Serve(old)
		return w.syncCode("2026-10-03T00:00:01Z")
	}),
	syncCase("SYNC_ROLLBACK", func(t *testing.T, w world) string {
		w.s.Clock.Set("2026-10-02T00:00:00Z")
		old, _ := w.s.LatestListing(t)
		w.sync(t, "2026-10-02T13:00:00Z")
		w.c.Hub.Tamper = testkit.Serve(old)
		return w.syncCode("2026-10-02T20:00:00Z")
	}),
	syncCase("SYNC_SEQUENCE_CONFLICT", func(t *testing.T, w world) string {
		w.sync(t, "2026-10-02T00:00:00Z")
		w.c.Hub.Tamper = testkit.Serve(w.s.Forge(t, func(l *lifecycle.Listing) { l.ExpiresAt++ }))
		return w.syncCode("2026-10-02T06:00:00Z")
	}),
	syncCase("SYNC_REVOCATION_DROPPED", func(t *testing.T, w world) string {
		w.sync(t, "2026-11-05T00:00:00Z")
		w.s.Clock.Set("2026-11-10T00:00:00Z")
		_, _ = w.s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
		w.sync(t, "2026-11-10T00:00:00Z")
		w.c.Hub.Tamper = testkit.Serve(w.s.Forge(t, func(l *lifecycle.Listing) { l.Sequence++; l.Versions = l.Versions[1:] }))
		return w.syncCode("2026-11-10T06:00:00Z")
	}),
	syncCase("SYNC_DATES_CHANGED", func(t *testing.T, w world) string {
		w.sync(t, "2026-10-10T00:00:00Z")
		w.c.Hub.Tamper = testkit.Serve(w.s.Forge(t, func(l *lifecycle.Listing) {
			l.Sequence++
			l.Versions[1].EnforcedFrom = l.Versions[1].EnforcedFrom.Add(-timex.Day)
		}))
		return w.syncCode("2026-10-10T06:00:00Z")
	}),
	syncCase("SYNC_DATES_INVALID", func(t *testing.T, w world) string {
		w.s.Clock.Set("2026-10-10T00:00:00Z")
		w.c.Hub.Tamper = testkit.Serve(w.s.Forge(t, func(l *lifecycle.Listing) { l.Versions[1].PublishedAt = l.Versions[1].NewContentFrom.Add(-timex.Day) }))
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_MANIFEST_DIGEST_MISMATCH", func(t *testing.T, w world) string {
		w.c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) { f.Manifest = append(f.Manifest, ' ') }
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_MANIFEST_SIGNATURE_INVALID", func(t *testing.T, w world) string {
		v1, v2 := w.manifest(t, "1.0.0"), w.manifest(t, "2.0.0")
		sig1, _ := w.s.Hub.Publication.PackageFile(v1.Segment(), "manifest.sig")
		w.c.Hub.TamperPkg = func(d ni.Digest, f *pkgv.Files) {
			if d == v2 {
				f.Sig = sig1
			}
		}
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_RECEIPT_INVALID", func(t *testing.T, w world) string {
		v1, v2 := w.manifest(t, "1.0.0"), w.manifest(t, "2.0.0")
		rc1, _ := w.s.Hub.Publication.PackageFile(v1.Segment(), "manifest.receipt")
		w.c.Hub.TamperPkg = func(d ni.Digest, f *pkgv.Files) {
			if d == v2 {
				f.Receipt = rc1
			}
		}
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_FILE_HASH_MISMATCH", func(t *testing.T, w world) string {
		w.c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) {
			for p, b := range f.Content {
				f.Content[p] = bytes.Replace(b, []byte("object"), []byte("objecT"), 1)
			}
		}
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_FILE_SET_MISMATCH", func(t *testing.T, w world) string {
		w.c.Hub.TamperPkg = func(_ ni.Digest, f *pkgv.Files) { f.Content["extra.ttl"] = []byte("x") }
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	syncCase("SYNC_INCOHERENT", func(t *testing.T, w world) string {
		w.s.Clock.Set("2026-10-10T00:00:00Z")
		v1 := w.manifest(t, "1.0.0")
		w.c.Hub.Tamper = testkit.Serve(w.s.Forge(t, func(l *lifecycle.Listing) { l.Versions[1].Manifest = v1 }))
		return w.syncCode("2026-10-10T00:00:00Z")
	}),
	eventCase("DID_KEY_ROTATED", func(t *testing.T, w world) {
		w.sync(t, "2026-10-02T00:00:00Z")
		kid := w.s.Signer.AddUnpublished()
		w.s.Signer.Publish(kid)
		_ = w.s.Signer.Activate(kid)
		w.s.Clock.Set("2026-10-02T00:05:00Z")
		_, _ = w.s.Hub.Listing.Issue(w.s.Clock.Now())
		_ = w.c.Sync.Run()
	}),
	eventCase("DID_REFRESH_FAILED", func(t *testing.T, w world) {
		w.sync(t, "2026-10-02T00:00:00Z")
		w.c.Resolver.Down = true
		_ = w.syncCode("2026-10-02T06:00:00Z")
	}),
	{"CLIENT_NOT_READY", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		return w.request(testkit.ContractRequest("odrl:use")), w.c.Events
	}},
	{"CLIENT_DEGRADED_REJECT", func(t *testing.T) (string, *testkit.Recorder) {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftWith(testkit.DraftV1, `{"degraded":{"policy_intake":"reject"}}`), app.ApproveOptions{})
		w := world{s, testkit.NewClient(s, testkit.F6Routes())}
		w.sync(t, "2026-09-10T00:00:00Z")
		w.c.Hub.Down = true
		w.s.Clock.Set("2026-09-11T00:00:00Z")
		return w.request(testkit.ContractRequest("odrl:use")), w.c.Events
	}},
	{"CLIENT_FAIL_CLOSED", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-10-02T00:00:00Z")
		w.c.Hub.Down = true
		w.s.Clock.Set("2026-10-10T00:00:00Z")
		return w.request(testkit.ContractRequest("odrl:use")), w.c.Events
	}},
	{"NO_VERSION_IN_FORCE", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-09-02T00:00:00Z")
		return w.request(testkit.ContractRequest("odrl:use")), w.c.Events
	}},
	{"MESSAGE_UNPARSEABLE", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-09-10T00:00:00Z")
		return w.request([]byte(`{`)), w.c.Events
	}},
	{"CONTEXT_NOT_IN_PACKAGE", func(t *testing.T) (string, *testkit.Recorder) {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		s.Publish(t, "2026-09-01T10:00:00Z", []byte(`{"space":"`+testkit.Space+`","version":"1.0.0","slots":[{"slot":"odrl","obligation":"mandatory"}],"files":[
{"path":"shapes.ttl","media_type":"text/turtle","role":"shacl","slot":"odrl","applies_to":["policy_intake"],"content":"@prefix sh: <http://www.w3.org/ns/shacl#> ."}]}`), app.ApproveOptions{})
		rs := testkit.F6Routes()[:1]
		rs[0].Body = nil
		w := world{s, testkit.NewClient(s, rs)}
		w.sync(t, "2026-09-10T00:00:00Z")
		return w.request([]byte(`{"@context":"https://evil.example/ctx.jsonld","@type":"ContractRequestMessage"}`)), w.c.Events
	}},
	{"VERDICT_VIOLATION", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-10-10T12:00:00Z")
		return w.request(testkit.ContractRequest("ex:undefined")), w.c.Events
	}},
	{"VERDICT_NOT_IN_HEAD", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-10-10T12:00:00Z")
		r := w.c.Send(domain.DSPIngress, "POST", "/negotiations/request", testkit.ContractRequest("ex:access"))
		if strings.Contains(testkit.VerdictOf(r), `code="VERDICT_NOT_IN_HEAD"`) {
			return "VERDICT_NOT_IN_HEAD", w.c.Events
		}
		return "", w.c.Events
	}},
	{"VERDICT_RECOMMENDED_NONCONFORMANT", func(t *testing.T) (string, *testkit.Recorder) {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		s.Publish(t, "2026-09-01T10:00:00Z", []byte(`{"space":"`+testkit.Space+`","version":"1.0.0","slots":[{"slot":"odrl","obligation":"optional"},{"slot":"dcat","obligation":"recommended"}],"files":[
{"path":"c.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["catalog_entry"],"content":{"required":["license"]}}]}`), app.ApproveOptions{})
		route := domain.Route{Listener: domain.DSPIngress, Match: domain.Match{Method: "POST", Path: "/catalog/request"}, Message: "CatalogRequestMessage", Gates: []domain.Gate{domain.GateCatalog}, Mode: domain.Enforce}
		w := world{s, testkit.NewClient(s, []domain.Route{route})}
		w.sync(t, "2026-09-10T00:00:00Z")
		w.c.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{}`))
		return firstEventCode(w.c.Events, "VERDICT_RECOMMENDED_NONCONFORMANT"), w.c.Events
	}},
	{"ROUTE_DEFAULT_REJECT", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		_ = w.c.Configure(testkit.F6Routes(), "reject", "forward")
		return testkit.BodyCode(w.c.Send(domain.DSPIngress, "POST", "/not/configured", []byte(`{}`)).Body), w.c.Events
	}},
	{"AGREEMENT_VERSION_REVOKED", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		w.sync(t, "2026-10-10T12:00:00Z")
		w.c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A1", "ex:access", ""))
		w.s.Clock.Set("2026-11-10T00:00:00Z")
		_, _ = w.s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
		w.sync(t, "2026-11-10T00:00:00Z")
		w.s.Clock.Set("2026-11-12T00:00:10Z")
		return testkit.BodyCode(w.c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:A1")).Body), w.c.Events
	}},
	{"AGREEMENT_UNKNOWN", func(t *testing.T) (string, *testkit.Recorder) {
		w := f6World(t)
		_ = w.c.Configure(testkit.F6Routes(), "forward", "reject")
		w.sync(t, "2026-10-10T12:00:00Z")
		return testkit.BodyCode(w.c.Send(domain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:uuid:nobody")).Body), w.c.Events
	}},
	eventCase("AGREEMENT_PROFILE_MISMATCH", func(t *testing.T, w world) {
		w.sync(t, "2026-11-05T12:00:00Z")
		w.c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A9", "odrl:use", "https://example.com/profiles/odrl/0.1.0"))
	}),
	eventCase("AGREEMENT_REVOCATION_SCHEDULED", func(t *testing.T, w world) {
		w.sync(t, "2026-10-10T12:00:00Z")
		w.c.Send(domain.DSPEgress, "POST", "/negotiations/c-1/agreement", testkit.AgreementMessage("urn:uuid:A1", "ex:access", ""))
		w.s.Clock.Set("2026-11-10T00:00:00Z")
		_, _ = w.s.Hub.Revocation.Revoke(testkit.Releaser, "1.0.0", app.RevokeOptions{Justification: "x"})
		w.sync(t, "2026-11-10T00:00:00Z")
	}),
	{"AUDIT_WILL_WARN", auditCase("AUDIT_WILL_WARN")},
	{"AUDIT_WILL_REJECT", auditCase("AUDIT_WILL_REJECT")},
	eventCase("CLOCK_SKEW_EXCEEDED", func(t *testing.T, w world) {
		w.s.Clock.Set("2026-10-02T00:00:00Z")
		ahead := w.s.Clock.Now().Add(30)
		w.c.Hub.Date = &ahead
		_ = w.c.Sync.Run()
	}),
}

// auditCase observes a catalog response that the next version deprecates and later removes.
func auditCase(code string) func(t *testing.T) (string, *testkit.Recorder) {
	return func(t *testing.T) (string, *testkit.Recorder) {
		draft := func(v, licenses string) []byte {
			return []byte(`{"space":"` + testkit.Space + `","version":"` + v + `","slots":[{"slot":"dcat","obligation":"mandatory"}],"files":[
{"path":"c.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["catalog_entry"],"content":{"properties":{"license":{"enum":` + licenses + `}}}}]}`)
		}
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		s.Publish(t, "2026-09-01T10:00:00Z", draft("1.0.0", `["cc-by","odc-by"]`), app.ApproveOptions{})
		route := domain.Route{Listener: domain.DSPIngress, Match: domain.Match{Method: "POST", Path: "/catalog/request"}, Message: "CatalogRequestMessage",
			Gates: []domain.Gate{domain.GateDSP}, Mode: domain.Shadow, ValidateResponse: &domain.ValidateResponse{Gate: domain.GateCatalog, Mode: domain.Shadow}}
		w := world{s, testkit.NewClient(s, []domain.Route{route})}
		w.sync(t, "2026-09-10T00:00:00Z")
		w.c.Upstream.Body = []byte(`{"license":"odc-by"}`)
		w.c.Send(domain.DSPIngress, "POST", "/catalog/request", []byte(`{}`))
		s.Publish(t, "2026-10-01T10:00:00Z", draft("2.0.0", `["cc-by"]`), app.ApproveOptions{})
		w.sync(t, "2026-10-01T10:00:00Z")
		return firstEventCode(w.c.Events, code), w.c.Events
	}
}

// firstEventCode returns code if some event carries it.
func firstEventCode(r *testkit.Recorder, code string) string {
	if r.Has(code) {
		return code
	}
	return ""
}

func TestN_ClientCodes(t *testing.T) {
	for _, c := range clientCases {
		t.Run("N-"+c.code, func(t *testing.T) {
			got, ev := c.run(t)
			if got != c.code {
				t.Fatalf("got %q want %s; events %v", got, c.code, ev.Codes())
			}
		})
	}
}
