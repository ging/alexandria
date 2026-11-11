// Longer N-<CODE> cases and the LIFECYCLE_* drafts of the negative corpus, kept apart from the case table for readability.
// Fixtures: testkit hubs and clients; each helper returns the codes it observed (DoD 3, F5 §5.2).
package conformance

import (
	"bytes"
	"encoding/json"
	"testing"

	capp "github.com/ging/alexandria/internal/client/app"
	cdomain "github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// lifecycleDrafts maps each LIFECYCLE_* code to the lifecycle object of its F5 negative case.
var lifecycleDrafts = map[codes.Code]string{
	codes.LifecycleStalenessLtTTL: `{"freshness":{"ttl":"PT24H","max_staleness":"PT12H"}}`,
	codes.LifecycleMinGrace:       `{"min_grace_window":"P1D"}`,
	codes.LifecycleGraceTooShort:  `{"grace_window":"P5D"}`,
	codes.LifecycleSyncInterval:   `{"sync_interval":"PT13H"}`,
	codes.LifecycleClockSkew:      `{"clock_skew":"PT0S"}`,
	codes.LifecycleLease:          `{"clock_skew":"PT10S","lease_margin":"PT5S"}`,
	codes.LifecyclePropagation:    `{"propagation_window":"PT24H","min_grace_window":"PT24H"}`,
	codes.LifecycleMaxVersions:    `{"max_concurrent_versions":1}`,
}

// init adds one case per LIFECYCLE_* code.
func init() {
	for c, lc := range lifecycleDrafts {
		cases[c] = func(t *testing.T) []string {
			_, err := hub(t).Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftWith(testkit.DraftV1, lc))
			return []string{codeOf(err)}
		}
	}
}

// testkitKey returns the did:web key of the hub signer under fragment.
func testkitKey(s *testkit.Server, fragment string) did.PublicKey {
	return did.PublicKey{Kid: string(testkit.WebDID) + fragment, Key: s.Signer.PublicKey()}
}

// ptrInstant parses an instant into a pointer.
func ptrInstant(s string) *timex.Instant { i := timex.MustInstant(s); return &i }

// manifestSignatureInvalid serves a manifest with an altered byte and the listing digest recomputed (S9).
func manifestSignatureInvalid(t *testing.T) []string {
	s := hub(t)
	s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
	c := testkit.NewClient(t, s)
	raw, _ := s.Hub.Listing.Latest(testkit.NS)
	var doc map[string]any
	_ = json.Unmarshal(raw, &doc)
	orig := ni.Digest(vers(doc)[0].(map[string]any)["manifest"].(string))
	f, _ := c.Hub.Package(orig)
	f.Manifest = bytes.Replace(f.Manifest, []byte(`"role":"other"`), []byte(`"role":"shacl"`), 1)
	bad := ni.Sum(f.Manifest)
	c.Hub.PackageHook = func(d ni.Digest) (pkgv.Files, bool) { return f, d == bad }
	forge(t, s, c, func(d map[string]any) { vers(d)[0].(map[string]any)["manifest"] = string(bad) })
	return []string{syncCode(c)}
}

// contextNotInPackage sends a message whose @context is not declared by the package of a SHACL slot.
func contextNotInPackage(t *testing.T) []string {
	s := hub(t)
	doc := []byte(`{"version":"1.0.0","slots":[{"slot":"odrl","obligation":"mandatory","applies_to":["policy_intake"]}],"files":[{"path":"odrl/shapes.ttl","media_type":"text/turtle","role":"shacl","slot":"odrl","applies_to":["policy_intake"],"content":"@prefix sh: <http://www.w3.org/ns/shacl#> ."}]}`)
	s.Publish(t, "2026-09-01T10:00:00Z", doc, app.ApproveOptions{})
	s.Clock.Set("2026-09-10T00:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	body := []byte(`{"@context":"https://evil.example/ctx.jsonld","@type":"ContractRequestMessage","providerPid":"urn:uuid:p-1"}`)
	return []string{testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, body).Body)}
}

// recommendedNonconformant publishes a recommended dcat slot that the message violates.
func recommendedNonconformant(t *testing.T) []string {
	s := hub(t)
	doc := []byte(`{"version":"1.0.0","slots":[{"slot":"dcat","obligation":"recommended","applies_to":["policy_intake"]}],"files":[{"path":"d.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["policy_intake"],"content":{"required":["title"]}}]}`)
	s.Publish(t, "2026-09-01T10:00:00Z", doc, app.ApproveOptions{})
	s.Clock.Set("2026-09-10T00:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, testkit.ContractRequest("odrl:use"))
	return c.Events.Codes()
}

// revocationScheduled registers A1 with 1.0.0 and revokes 1.0.0.
func revocationScheduled(t *testing.T) []string {
	s, c := synced(t, "2026-10-10T12:00:00Z")
	c.SetMode(t, "contract-agreement-in", cdomain.Enforce)
	c.Send(cdomain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
	s.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"}); err != nil {
		t.Fatal(err)
	}
	c.SyncNow(t)
	return c.Events.Codes()
}

// audit observes a catalog response and publishes a breaking version that first warns then rejects it.
func audit(t *testing.T) []string {
	s := hub(t)
	catalog := func(v, enum string) []byte {
		return []byte(`{"version":"` + v + `","slots":[{"slot":"dcat","obligation":"mandatory","applies_to":["catalog_entry"]}],"files":[{"path":"c.json","media_type":"application/schema+json","role":"json-schema","slot":"dcat","applies_to":["catalog_entry"],"content":{"properties":{"action":{"enum":` + enum + `}}}}]}`)
	}
	s.Publish(t, "2026-09-01T10:00:00Z", catalog("1.0.0", `["a","b"]`), app.ApproveOptions{})
	s.Clock.Set("2026-09-10T00:00:00Z")
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	c.Upstream.Body = []byte(`{"action":"b"}`)
	c.Send(cdomain.DSPIngress, "POST", "/catalog/request", []byte(`{"@type":"CatalogRequestMessage"}`))
	s.Publish(t, "2026-09-20T10:00:00Z", catalog("2.0.0", `["a"]`), app.ApproveOptions{})
	c.SyncNow(t)
	return c.Events.Codes()
}

// clockSkew syncs against a hub whose Date is 30 seconds behind the client.
func clockSkew(t *testing.T) []string {
	_, c := synced(t, "2026-10-10T12:00:00Z")
	c.Status.ObserveSkew(c.Server.Clock.Now()-30, c.Server.Clock.Now())
	return c.Events.Codes()
}

// appRequest converts a domain request into an app request.
func appRequest(r cdomain.Request) *capp.Request {
	return &capp.Request{Listener: r.Listener, Method: r.Method, Path: r.Path, Body: r.Body}
}
