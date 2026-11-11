// Negative corpus N-<CODE>: one minimal case per reachable F5 code, each asserting the code it must trigger (DoD 3, F5 §5.2).
// Hub and client cases run in process on the testkit doubles; startup codes run the real alexandria-client binary.
package conformance

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	cdomain "github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// unreachable lists the codes the PoC cannot trigger, each explained in OPEN_QUESTIONS.md.
var unreachable = map[codes.Code]string{codes.VerdictShaclWarning: "DP-7: SHACL shapes are not evaluated"}

// codeOf returns the code of an error, or of the first finding.
func codeOf(err error) string {
	var f app.Findings
	if errors.As(err, &f) {
		return string(f[0].Code)
	}
	if ce, ok := codes.As(err); ok {
		return string(ce.Code)
	}
	return ""
}

// hub returns an F6 hub with namespace demo.
func hub(t *testing.T) *testkit.Server { return testkit.NewServer(t, "2026-08-01T00:00:00Z") }

// f6Hub returns a hub with both F6 releases.
func f6Hub(t *testing.T) *testkit.Server { s := hub(t); s.PublishF6(t); return s }

// synced returns an F6 hub and a client synced at ts.
func synced(t *testing.T, ts string) (*testkit.Server, *testkit.Client) {
	s := f6Hub(t)
	s.Clock.Set(ts)
	c := testkit.NewClient(t, s)
	c.SyncNow(t)
	return s, c
}

// syncCode runs a sync and returns its code.
func syncCode(c *testkit.Client) string {
	if err := c.Sync.Run(); err != nil {
		return string(err.Code)
	}
	return ""
}

// forge re-signs the listing served to c after mutate.
func forge(t *testing.T, s *testkit.Server, c *testkit.Client, mutate func(map[string]any)) {
	c.Hub.ListingHook = func(b []byte) []byte { return s.Resign(t, b, mutate) }
}

// vers returns the versions of a decoded listing.
func vers(d map[string]any) []any { return testkit.Subject(d)["versions"].([]any) }

// propose creates and proposes doc in demo.
func propose(t *testing.T, s *testkit.Server, doc []byte) string { return s.Propose(t, doc) }

// grace returns a pointer to a duration.
func grace(d timex.Duration) *timex.Duration { return &d }

// msgBody posts an F6 contract request and returns the reject code of the body.
func msgBody(c *testkit.Client, action string) string {
	return testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, testkit.ContractRequest(action)).Body)
}

// startClient runs the client binary with a configuration and returns its stderr.
func startClient(t *testing.T, cfg string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "client.json")
	_ = os.WriteFile(p, []byte(cfg), 0o600)
	var stderr bytes.Buffer
	cmd := exec.Command(bin("alexandria-client"), "-config", p, "-state-dir", filepath.Join(dir, "state")) // #nosec G204 -- test binary.
	cmd.Stderr = &stderr
	if err := cmd.Run(); err == nil {
		t.Fatal("the client started with an invalid configuration")
	}
	return stderr.String()
}

// clientConfig is a valid configuration with a placeholder for extra root fields.
const clientConfig = `{"trust":{"hub":"https://127.0.0.1:1","namespace":"demo","hub_did":"did:web:h"},"upstreams":{"dsp":"http://127.0.0.1:2"},"listeners":{"admin":{"bind":"127.0.0.1:0"}},"events":{"sink":"file:/dev/null"}%s}`

// cases maps each reachable code to its minimal case; it returns the observed codes.
var cases = map[codes.Code]func(t *testing.T) []string{
	codes.APIUnauthenticated: func(t *testing.T) []string {
		_, err := hub(t).Hub.Namespaces.Create(nil, app.NamespaceInput{Slug: "x1"})
		return []string{codeOf(err)}
	},
	codes.APIForbiddenRole: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "b1"})
		_, err := s.Hub.Release.Approve(testkit.Sergio, "b1", "c", app.ApproveOptions{})
		return []string{codeOf(err)}
	},
	codes.APIPreconditionRequired: func(t *testing.T) []string {
		s := hub(t)
		d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
		_, err := s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, "", testkit.DraftV1)
		return []string{codeOf(err)}
	},
	codes.APIPreconditionFailed: func(t *testing.T) []string {
		s := hub(t)
		d, _ := s.Hub.Authoring.CreateDraft(testkit.Sergio, testkit.NS, testkit.DraftV1)
		_, _ = s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, d.ETag, testkit.DraftV1)
		_, err := s.Hub.Authoring.ReplaceDraft(testkit.Sergio, testkit.NS, d.ID, d.ETag, testkit.DraftV1)
		return []string{codeOf(err)}
	},
	codes.APINotFound: func(t *testing.T) []string {
		_, err := hub(t).Hub.Namespaces.Get("no-existe")
		return []string{codeOf(err)}
	},
	codes.APIInvalidState: func(t *testing.T) []string {
		s := hub(t)
		id := propose(t, s, testkit.DraftV1)
		_, _ = s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
		return []string{codeOf(err)}
	},
	codes.NSSlugInvalid: func(t *testing.T) []string {
		_, err := hub(t).Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "Demo_1"})
		return []string{codeOf(err)}
	},
	codes.NSExists: func(t *testing.T) []string {
		_, err := hub(t).Hub.Namespaces.Create(testkit.Ana, app.NamespaceInput{Slug: "demo"})
		return []string{codeOf(err)}
	},
	codes.StewardExists: func(t *testing.T) []string {
		_, err := hub(t).Hub.Namespaces.AssignSteward(testkit.Ana, testkit.NS, app.StewardInput{Iss: testkit.Sergio.Iss, Sub: testkit.Sergio.Sub})
		return []string{codeOf(err)}
	},
	codes.VocabNameInvalid: func(t *testing.T) []string {
		_, err := hub(t).Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "Mi Vocab", Kind: "hosted", MediaType: "application/json"})
		return []string{codeOf(err)}
	},
	codes.VocabExists: func(t *testing.T) []string {
		s := hub(t)
		in := app.VocabInput{Name: "dcat-ap", Kind: "hosted", MediaType: "application/ld+json"}
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, in)
		_, err := s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, in)
		return []string{codeOf(err)}
	},
	codes.VocabKindMismatch: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "h", Kind: "hosted", MediaType: "application/json"})
		_, _, err := s.Hub.Vocabularies.Refresh(testkit.Sergio, testkit.NS, "h")
		return []string{codeOf(err)}
	},
	codes.VocabMediaTypeMismatch: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "h", Kind: "hosted", MediaType: "application/ld+json"})
		_, _, err := s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, "h", "text/turtle", "", []byte("."))
		return []string{codeOf(err)}
	},
	codes.VocabUnparseable: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "h", Kind: "hosted", MediaType: "application/schema+json"})
		_, _, err := s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, "h", "application/schema+json", "", []byte("{"))
		return []string{codeOf(err)}
	},
	codes.VocabTooLarge: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "h", Kind: "hosted", MediaType: "text/plain"})
		_, _, err := s.Hub.Vocabularies.PutContent(testkit.Sergio, testkit.NS, "h", "text/plain", "", make([]byte, 1<<17))
		return []string{codeOf(err)}
	},
	codes.VocabUpstreamFailed: func(t *testing.T) []string {
		s := hub(t)
		_, _ = s.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "p", Kind: "proxy", MediaType: "application/ld+json", Upstream: "https://u.example/c"})
		s.Fetcher.Err = errors.New("500")
		_, _, err := s.Hub.Vocabularies.Refresh(testkit.Sergio, testkit.NS, "p")
		return []string{codeOf(err)}
	},
	codes.ReleaseAborted: func(t *testing.T) []string {
		s := hub(t)
		id := propose(t, s, testkit.DraftV1)
		s.DB.Faults.BlobPut = errors.New("disk")
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
		return []string{codeOf(err)}
	},
	codes.SignerKeyNotPublished: func(t *testing.T) []string {
		s := hub(t)
		id := propose(t, s, testkit.DraftV1)
		s.Signer.Rotate(9, "did:key:z6MkOther#z6MkOther")
		s.Hub.Keys.CheckSigningKey()
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
		return []string{codeOf(err)}
	},
	codes.DatesGraceOnCompatible: func(t *testing.T) []string {
		s := hub(t)
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
		id := propose(t, s, testkit.Draft("1.1.0", "odrl:use"))
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{GraceWindow: grace(10 * timex.Day)})
		return []string{codeOf(err)}
	},
	codes.DatesGraceTooShort: func(t *testing.T) []string {
		s := hub(t)
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
		id := propose(t, s, testkit.DraftV2)
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{GraceWindow: grace(3 * timex.Day)})
		return []string{codeOf(err)}
	},
	codes.DatesJustificationRequired: func(t *testing.T) []string {
		s := f6Hub(t)
		_, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{})
		return []string{codeOf(err)}
	},
	codes.DatesNonMonotonic: func(t *testing.T) []string {
		s := f6Hub(t)
		s.Clock.Set("2026-10-05T10:00:00Z")
		id := propose(t, s, testkit.Draft("3.0.0", "odrl:use"))
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{GraceWindow: grace(7 * timex.Day)})
		return []string{codeOf(err)}
	},
	codes.DatesTooManyVersions: func(t *testing.T) []string {
		s := hub(t)
		lc := `{"max_concurrent_versions":2}`
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftWith(testkit.DraftV1, lc), app.ApproveOptions{})
		s.Publish(t, "2026-10-01T10:00:00Z", testkit.DraftWith(testkit.DraftV2, lc), app.ApproveOptions{})
		s.Clock.Set("2026-10-05T10:00:00Z")
		id := propose(t, s, testkit.DraftWith(testkit.Draft("2.1.0", "odrl:use"), lc))
		_, err := s.Hub.Release.Approve(testkit.Sergio, testkit.NS, id, app.ApproveOptions{})
		return []string{codeOf(err)}
	},
	codes.DatesRevocationTooEarly: func(t *testing.T) []string {
		s := f6Hub(t)
		rf := s.Clock.Now().Add(timex.Day)
		_, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x", RevokedFrom: &rf})
		return []string{codeOf(err)}
	},
	codes.DatesRevokeLeavesNoHead: func(t *testing.T) []string {
		s := hub(t)
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
		_, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
		return []string{codeOf(err)}
	},
	codes.DatesAlreadyRevoked: func(t *testing.T) []string {
		s := f6Hub(t)
		s.Clock.Set("2026-11-10T00:00:00Z")
		_, _ = s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
		_, err := s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
		return []string{codeOf(err)}
	},
	codes.ConfigSchema: func(t *testing.T) []string {
		return []string{firstCode(startClient(t, strings.Replace(clientConfig, "%s", `,"unknown":1`, 1)))}
	},
	codes.ConfigRulesSeedUnreadable: func(t *testing.T) []string {
		return []string{firstCode(startClient(t, strings.Replace(clientConfig, "%s", `,"rules":{"seed":"file:///no/existe"}`, 1)))}
	},
	codes.RulesSchema: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		_, err := c.Rules.Add(`"1"`, []byte(`{"id":"x","priority":1,"listener":"dsp-ingress","match":{"method":"POST","path":"/x"},"message":"ContractRequestMessage","verify":{}}`))
		return []string{codeOf(err)}
	},
	codes.RulesDuplicateID: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		_, err := c.Rules.Add(`"1"`, []byte(`{"id":"contract-request","priority":999,"listener":"dsp-ingress","match":{"method":"POST","path":"/x"},"message":"ContractRequestMessage","verify":{"gates":["dsp_message"]}}`))
		return []string{codeOf(err)}
	},
	codes.RulesDuplicatePriority: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		_, err := c.Rules.Add(`"1"`, []byte(`{"id":"x","priority":100,"listener":"dsp-ingress","match":{"method":"POST","path":"/x"},"message":"ContractRequestMessage","verify":{"gates":["dsp_message"]}}`))
		return []string{codeOf(err)}
	},
	codes.RulesGateMessageMismatch: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		_, err := c.Rules.Add(`"1"`, []byte(`{"id":"x","priority":5,"listener":"dsp-ingress","match":{"method":"POST","path":"/x"},"message":"ContractRequestMessage","verify":{"gates":["agreement_admission"]}}`))
		return []string{codeOf(err)}
	},
	codes.RulesUpstreamMissing: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		_, err := c.Rules.Add(`"1"`, []byte(`{"id":"x","priority":5,"listener":"mgmt-ingress","match":{"method":"POST","path":"/x"},"message":"management","verify":{"gates":["dsp_message"]}}`))
		return []string{codeOf(err)}
	},
	codes.SyncUnreachable: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		c.Hub.Down = true
		return []string{syncCode(c)}
	},
	codes.SyncNamespaceNotFound: func(t *testing.T) []string { return []string{syncCode(testkit.NewClient(t, hub(t)))} },
	codes.SyncMalformed: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		forge(t, s, c, func(d map[string]any) { d["issuer"] = "did:key:z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH" })
		return []string{syncCode(c)}
	},
	codes.SyncSignatureInvalid: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		c.Hub.ListingHook = func(b []byte) []byte { return bytes.Replace(b, []byte(`"sequence":`), []byte(`"sequence":1`), 1) }
		return []string{syncCode(c)}
	},
	codes.SyncDidUnresolvable: func(t *testing.T) []string {
		s := testkit.NewWebServer(t, "2026-08-01T00:00:00Z")
		s.PublishF6(t)
		s.Resolver.Fail = true
		return []string{syncCode(testkit.NewClient(t, s))}
	},
	codes.DidKeyRotated: func(t *testing.T) []string {
		s := testkit.NewWebServer(t, "2026-08-01T00:00:00Z")
		s.PublishF6(t)
		c := testkit.NewClient(t, s)
		c.SyncNow(t)
		s.Signer.Rotate(7, string(testkit.WebDID)+"#key-2")
		s.Resolver.SetDocument(testkit.WebDID, testkitKey(s, "#key-2"))
		s.Hub.Keys.CheckSigningKey()
		s.Clock.Advance(120)
		_, _ = s.Hub.Listing.Issue(testkit.NS, s.Clock.Now())
		_ = c.Sync.Run()
		return c.Events.Codes()
	},
	codes.DidRefreshFailed: func(t *testing.T) []string {
		s := testkit.NewWebServer(t, "2026-08-01T00:00:00Z")
		s.PublishF6(t)
		c := testkit.NewClient(t, s)
		c.SyncNow(t)
		s.Resolver.Fail = true
		s.Clock.Advance(13 * 3600)
		_ = c.Sync.Run()
		return c.Events.Codes()
	},
	codes.SyncExpired: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		forge(t, s, c, func(d map[string]any) { d["validUntil"] = "2026-10-10T11:00:00Z" })
		return []string{syncCode(c)}
	},
	codes.SyncRollback: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["sequence"] = float64(1) })
		return []string{syncCode(c)}
	},
	codes.SyncSequenceConflict: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		forge(t, s, c, func(d map[string]any) { d["validUntil"] = "2026-10-11T13:00:00Z" })
		return []string{syncCode(c)}
	},
	codes.SyncRevocationDropped: func(t *testing.T) []string {
		s, c := synced(t, "2026-11-10T00:00:00Z")
		_, _ = s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
		c.SyncNow(t)
		s.Clock.Set("2026-11-10T13:00:00Z")
		forge(t, s, c, func(d map[string]any) { testkit.Subject(d)["versions"] = vers(d)[1:] })
		return []string{syncCode(c)}
	},
	codes.SyncDatesChanged: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		s.Clock.Set("2026-10-11T00:00:00Z")
		forge(t, s, c, func(d map[string]any) { vers(d)[1].(map[string]any)["enforced_from"] = "2026-11-01T10:00:00Z" })
		return []string{syncCode(c)}
	},
	codes.SyncDatesInvalid: func(t *testing.T) []string {
		s := f6Hub(t)
		c := testkit.NewClient(t, s)
		forge(t, s, c, func(d map[string]any) { vers(d)[1].(map[string]any)["enforced_from"] = "2026-10-06T10:00:00Z" })
		return []string{syncCode(c)}
	},
	codes.SyncManifestDigestMismatch: func(t *testing.T) []string {
		c := testkit.NewClient(t, f6Hub(t))
		c.Hub.ManifestHook = func(_ ni.Digest, m []byte) []byte { return append(m, ' ') }
		return []string{syncCode(c)}
	},
	codes.SyncManifestSignatureInvalid: manifestSignatureInvalid,
	codes.SyncFileHashMismatch: func(t *testing.T) []string {
		c := testkit.NewClient(t, f6Hub(t))
		c.Hub.FilesHook = func(f map[string][]byte) {
			f["schemas/policy.schema.json"] = append([]byte(" "), f["schemas/policy.schema.json"]...)
		}
		return []string{syncCode(c)}
	},
	codes.SyncFileSetMismatch: func(t *testing.T) []string {
		c := testkit.NewClient(t, f6Hub(t))
		c.Hub.FilesHook = func(f map[string][]byte) { f["extra.json"] = []byte(`{}`) }
		return []string{syncCode(c)}
	},
	codes.SyncIncoherent: func(t *testing.T) []string {
		s := f6Hub(t)
		c := testkit.NewClient(t, s)
		forge(t, s, c, func(d map[string]any) { vers(d)[1].(map[string]any)["version"] = "2.0.1" })
		return []string{syncCode(c)}
	},
	codes.ClientNotReady: func(t *testing.T) []string { return []string{msgBody(testkit.NewClient(t, f6Hub(t)), "odrl:use")} },
	codes.ClientDegradedReject: func(t *testing.T) []string {
		s := hub(t)
		doc := testkit.DraftWith(testkit.DraftV1, `{"degraded":{"policy_intake":"reject"}}`)
		s.Publish(t, "2026-09-01T10:00:00Z", doc, app.ApproveOptions{})
		s.Clock.Set("2026-09-10T00:00:00Z")
		c := testkit.NewClient(t, s)
		c.SyncNow(t)
		s.Clock.Set("2026-09-11T00:00:01Z")
		return []string{msgBody(c, "odrl:use")}
	},
	codes.ClientFailClosed: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		s.Clock.Set("2026-10-18T12:00:01Z")
		return []string{msgBody(c, "odrl:use")}
	},
	codes.NoVersionInForce: func(t *testing.T) []string {
		s := hub(t)
		s.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, app.ApproveOptions{})
		c := testkit.NewClient(t, s)
		c.SyncNow(t)
		return []string{msgBody(c, "odrl:use")}
	},
	codes.MessageUnparseable: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		return []string{testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, []byte("{")).Body)}
	},
	codes.ContextNotInPackage: contextNotInPackage,
	codes.VerdictViolation: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		return []string{msgBody(c, "ex:delete")}
	},
	codes.VerdictNotInHead: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		return []string{testkit.VerdictOf(c.Send(cdomain.DSPIngress, "POST", testkit.ContractRequestPath, testkit.ContractRequest("ex:access")))}
	},
	codes.VerdictRecommendedNonconformant: recommendedNonconformant,
	codes.RuleDefaultReject: func(t *testing.T) []string {
		s := f6Hub(t)
		s.Clock.Set("2026-10-10T12:00:00Z")
		c := testkit.NewClientWith(t, s, testkit.SeedPath(t), "reject", "forward")
		c.SyncNow(t)
		return []string{testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", "/nothing", []byte(`{}`)).Body)}
	},
	codes.AgreementVersionRevoked: func(t *testing.T) []string {
		s, c := synced(t, "2026-10-10T12:00:00Z")
		c.SetMode(t, "contract-agreement-in", cdomain.Enforce)
		c.SetMode(t, "transfer-request", cdomain.Enforce)
		c.Send(cdomain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", ""))
		s.Clock.Set("2026-11-10T00:00:00Z")
		_, _ = s.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"})
		c.SyncNow(t)
		s.Clock.Set("2026-11-12T00:00:10Z")
		return []string{testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:A1")).Body)}
	},
	codes.AgreementUnknown: func(t *testing.T) []string {
		s := f6Hub(t)
		s.Clock.Set("2026-10-10T12:00:00Z")
		c := testkit.NewClientWith(t, s, testkit.SeedPath(t), "forward", "reject")
		c.SyncNow(t)
		c.SetMode(t, "transfer-request", cdomain.Enforce)
		return []string{testkit.BodyCode(c.Send(cdomain.DSPIngress, "POST", "/transfers/request", testkit.TransferRequest("urn:none")).Body)}
	},
	codes.AgreementProfileMismatch: func(t *testing.T) []string {
		_, c := synced(t, "2026-10-10T12:00:00Z")
		c.Send(cdomain.DSPIngress, "POST", testkit.AgreementPath, testkit.AgreementMessage("urn:A1", "ex:access", "https://other/profile"))
		return c.Events.Codes()
	},
	codes.AgreementRevocationScheduled: func(t *testing.T) []string { return revocationScheduled(t) },
	codes.AuditWillWarn:                func(t *testing.T) []string { return audit(t) },
	codes.AuditWillReject:              func(t *testing.T) []string { return audit(t) },
	codes.ClockSkewExceeded:            func(t *testing.T) []string { return clockSkew(t) },
}

// firstCode extracts the code printed by the client at startup.
func firstCode(stderr string) string {
	_, rest, _ := strings.Cut(stderr, "alexandria-client: ")
	code, _, _ := strings.Cut(rest, ":")
	return code
}

func TestN_EveryReachableCode(t *testing.T) {
	for c, run := range cases {
		t.Run("N-"+string(c), func(t *testing.T) {
			if got := run(t); !contains(got, string(c)) {
				t.Fatalf("observed %v, want %s", got, c)
			}
		})
	}
}

func TestN_CatalogIsCovered(t *testing.T) {
	oq, err := os.ReadFile("../OPEN_QUESTIONS.md")
	if err != nil {
		t.Fatal(err)
	}
	for c := range codes.All() {
		_, has := cases[c]
		reason, skip := unreachable[c]
		switch {
		case has == skip:
			t.Errorf("%s must be either reachable or listed as unreachable", c)
		case skip && !bytes.Contains(oq, []byte(string(c))):
			t.Errorf("%s is unreachable (%s) but not explained in OPEN_QUESTIONS.md", c, reason)
		}
	}
}

// contains reports whether xs holds s or a string containing it.
func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s || strings.Contains(x, `"`+s+`"`) {
			return true
		}
	}
	return false
}
