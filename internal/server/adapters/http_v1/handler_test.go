// HTTP contract of /v1 (§3.9) over httptest with the real jwt_jwks verifier: authorization matrix, preconditions, caching and JSON-LD.
// Fixtures: the testkit HTTP hub, the F6 drafts and an offline JSON-LD loader with the A.5 and VC 2.0 contexts (T-HUB-012, 017, 018, 019, 029).
package http_v1_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/jsonld"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/server/adapters/compiler_dsl"
	"github.com/ging/alexandria/internal/testkit"
)

// hub returns an HTTP hub with namespace demo and steward Sergio.
func hub(t *testing.T) *testkit.HTTPServer {
	t.Helper()
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.SetupHTTP(t)
	return h
}

// code extracts the problem code of a body.
func code(b []byte) string {
	var p struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(b, &p)
	return p.Code
}

// publish creates, proposes and approves doc over HTTP.
func publish(t *testing.T, h *testkit.HTTPServer, doc []byte) {
	t.Helper()
	st, hdr, b := h.Do(t, "POST", "/v1/namespaces/demo/drafts", h.SergioTok, doc, nil)
	if st != 201 {
		t.Fatalf("draft: %d %s", st, b)
	}
	id := hdr.Get("Location")[strings.LastIndex(hdr.Get("Location"), "/")+1:]
	st, _, b = h.Do(t, "POST", "/v1/namespaces/demo/drafts/"+id+":propose", h.SergioTok, nil, nil)
	if st != 201 {
		t.Fatalf("propose: %d %s", st, b)
	}
	var c struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(b, &c)
	cid := c.ID[strings.LastIndex(c.ID, "/")+1:]
	if st, _, b = h.Do(t, "POST", "/v1/namespaces/demo/candidates/"+cid+":approve", h.SergioTok, nil, nil); st != 201 {
		t.Fatalf("approve: %d %s", st, b)
	}
}

// loader returns an offline JSON-LD loader with the VC 2.0 context and the given A.5 context bytes.
func loader(t *testing.T, ctx []byte) *jsonld.Loader {
	t.Helper()
	l := jsonld.NewLoader()
	if err := l.Add(testkit.Context, ctx); err != nil {
		t.Fatal(err)
	}
	vc2, err := os.ReadFile("../../../../conformance/testdata/credentials-v2.jsonld")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.Add("https://www.w3.org/ns/credentials/v2", vc2); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestT_HUB_017_NoTokenIs401WithChallenge(t *testing.T) {
	h := hub(t)
	st, hdr, b := h.Do(t, "POST", "/v1/namespaces", "", []byte(`{"slug":"x1"}`), nil)
	if st != 401 || code(b) != "API_UNAUTHENTICATED" || hdr.Get("WWW-Authenticate") != "Bearer" {
		t.Fatalf("%d %s %q", st, b, hdr.Get("WWW-Authenticate"))
	}
}

func TestT_HUB_022_InvalidTokenIs401InvalidToken(t *testing.T) {
	h := hub(t)
	st, hdr, _ := h.Do(t, "GET", "/v1/namespaces", "not-a-jwt", nil, nil)
	if st != 401 || hdr.Get("WWW-Authenticate") != `Bearer error="invalid_token"` {
		t.Fatalf("%d %q", st, hdr.Get("WWW-Authenticate"))
	}
}

func TestT_HUB_017_Matrix(t *testing.T) {
	h := hub(t)
	cases := []struct {
		method, path, tok string
		want              int
	}{
		{"GET", "/v1/namespaces/demo", "", 200},
		{"POST", "/v1/namespaces", h.SergioTok, 403},
		{"GET", "/v1/namespaces/demo/stewards", h.EveTok, 403},
		{"GET", "/v1/namespaces/demo/stewards", h.SergioTok, 200},
		{"GET", "/v1/namespaces/demo/stewards", h.AnaTok, 200},
		{"POST", "/v1/namespaces/demo/drafts", h.AnaTok, 403},
		{"POST", "/v1/namespaces/demo/drafts", "", 401},
		{"GET", "/v1/events", h.SergioTok, 403},
		{"GET", "/v1/events", h.AnaTok, 200},
		{"GET", "/v1/namespaces/demo/events", h.SergioTok, 200},
	}
	for _, c := range cases {
		if st, _, b := h.Do(t, c.method, c.path, c.tok, testkit.DraftV1, nil); st != c.want {
			t.Errorf("%s %s: %d %s", c.method, c.path, st, b)
		}
	}
}

func TestT_HUB_018_IfMatchRequiredAndChecked(t *testing.T) {
	h := hub(t)
	_, hdr, _ := h.Do(t, "POST", "/v1/namespaces/demo/drafts", h.SergioTok, testkit.DraftV1, nil)
	loc := strings.TrimPrefix(hdr.Get("Location"), testkit.Base)
	if st, _, b := h.Do(t, "PUT", loc, h.SergioTok, testkit.DraftV1, nil); st != 428 || code(b) != "API_PRECONDITION_REQUIRED" {
		t.Fatalf("no If-Match: %d %s", st, b)
	}
	if st, _, b := h.Do(t, "PUT", loc, h.SergioTok, testkit.DraftV1, map[string]string{"If-Match": `"stale"`}); st != 412 || code(b) != "API_PRECONDITION_FAILED" {
		t.Fatalf("stale If-Match: %d %s", st, b)
	}
}

func TestT_HUB_019_UnknownPathsAre404(t *testing.T) {
	h := hub(t)
	for _, p := range []string{"/v1/namespaces/no-existe", "/v1/namespaces/demo/versions/9.9.9", "/v1/nothing"} {
		if st, _, b := h.Do(t, "GET", p, h.SergioTok, nil, nil); st != 404 || code(b) != "API_NOT_FOUND" {
			t.Errorf("%s: %d %s", p, st, b)
		}
	}
}

func TestT_HUB_012_VersionsAreNoCache(t *testing.T) {
	h := hub(t)
	publish(t, h, testkit.DraftV1)
	for _, p := range []string{"/v1/namespaces/demo/versions", "/v1/namespaces/demo/versions/1.0.0"} {
		if st, hdr, _ := h.Do(t, "GET", p, "", nil, nil); st != 200 || hdr.Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: %d %q", p, st, hdr.Get("Cache-Control"))
		}
	}
}

func TestT_HUB_029_ResponsesExpandAsJSONLD(t *testing.T) {
	h := hub(t)
	publish(t, h, testkit.DraftV1)
	l := loader(t, schemas.Context())
	paths := []string{
		"/v1/namespaces", "/v1/namespaces/demo", "/v1/namespaces/demo/stewards", "/v1/namespaces/demo/events",
		"/v1/namespaces/demo/vocabularies", "/v1/namespaces/no-existe",
	}
	for _, p := range paths {
		_, hdr, b := h.Do(t, "GET", p, h.SergioTok, nil, nil)
		if ct := hdr.Get("Content-Type"); ct != "application/ld+json" && ct != "application/problem+json" {
			t.Errorf("%s: content type %s", p, ct)
		}
		if _, err := jsonld.Expand(b, l); err != nil {
			t.Errorf("%s does not expand: %v", p, err)
		}
	}
}

// withoutIssSub removes the two A.5 terms that redefine protected VC 2.0 terms (OPEN_QUESTIONS, G9).
func withoutIssSub(t *testing.T) []byte {
	t.Helper()
	var doc map[string]map[string]any
	_ = json.Unmarshal(schemas.Context(), &doc)
	delete(doc["@context"], "iss")
	delete(doc["@context"], "sub")
	b, _ := json.Marshal(doc)
	return b
}

func TestT_HUB_029_CredentialsConflictOnlyOnIssAndSub(t *testing.T) {
	h := hub(t)
	publish(t, h, testkit.DraftV1)
	for _, p := range []string{"/v1/namespaces/demo/versions", "/v1/namespaces/demo/versions/1.0.0"} {
		_, _, b := h.Do(t, "GET", p, "", nil, nil)
		if _, err := jsonld.Expand(b, loader(t, schemas.Context())); err == nil || !strings.Contains(err.Error(), "protected term") {
			t.Errorf("%s: expected the A.5/VC 2.0 protected-term conflict, got %v", p, err)
		}
		if _, err := jsonld.Expand(b, loader(t, withoutIssSub(t))); err != nil {
			t.Errorf("%s does not expand without iss/sub: %v", p, err)
		}
	}
}

// TestYAMLDraftDocumentRoundTripsAndExpands verifies that YAML DSL drafts return their source string on GET and expand as JSON-LD.
func TestYAMLDraftDocumentRoundTripsAndExpands(t *testing.T) {
	h := testkit.NewHTTPServerWith(t, "2026-09-01T10:00:00Z", compiler_dsl.Compiler{})
	h.SetupHTTP(t)
	yamlDoc, err := os.ReadFile("../../../../conformance/f6/drafts/v1.0.0.yaml")
	if err != nil {
		t.Fatal(err)
	}
	st, hdr, b := h.Do(t, "POST", "/v1/namespaces/demo/drafts", h.SergioTok, yamlDoc, map[string]string{"Content-Type": "application/yaml"})
	if st != 201 {
		t.Fatalf("create yaml draft: %d %s", st, b)
	}
	loc := strings.TrimPrefix(hdr.Get("Location"), testkit.Base)
	st, _, b = h.Do(t, "GET", loc, h.SergioTok, nil, nil)
	if st != 200 {
		t.Fatalf("get yaml draft: %d %s", st, b)
	}
	var out struct {
		Document string `json:"document"`
	}
	if err := json.Unmarshal(b, &out); err != nil || out.Document != string(yamlDoc) {
		t.Fatalf("document mismatch: err=%v got=%q", err, out.Document)
	}
	if _, err := jsonld.Expand(b, loader(t, schemas.Context())); err != nil {
		t.Fatalf("yaml draft response does not expand: %v", err)
	}
}

// TestCandidateFilesBeforeApproval verifies that an unapproved candidate lists its compiled files and serves their bytes.
func TestCandidateFilesBeforeApproval(t *testing.T) {
	h := hub(t)
	_, hdr, _ := h.Do(t, "POST", "/v1/namespaces/demo/drafts", h.SergioTok, testkit.DraftV1, nil)
	did := hdr.Get("Location")[strings.LastIndex(hdr.Get("Location"), "/")+1:]
	st, _, b := h.Do(t, "POST", "/v1/namespaces/demo/drafts/"+did+":propose", h.SergioTok, nil, nil)
	if st != 201 {
		t.Fatalf("propose: %d %s", st, b)
	}
	if _, err := jsonld.Expand(b, loader(t, schemas.Context())); err != nil {
		t.Fatalf("candidate response does not expand: %v", err)
	}
	var cand struct {
		ID    string `json:"id"`
		Files []struct {
			Path      string `json:"path"`
			MediaType string `json:"media_type"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &cand); err != nil || len(cand.Files) == 0 {
		t.Fatalf("expected candidate files: err=%v len=%d", err, len(cand.Files))
	}
	cid := cand.ID[strings.LastIndex(cand.ID, "/")+1:]
	first := cand.Files[0]
	st, fhdr, fb := h.Do(t, "GET", "/v1/namespaces/demo/candidates/"+cid+"/files/"+first.Path, h.SergioTok, nil, nil)
	if st != 200 || len(fb) == 0 || !strings.HasPrefix(fhdr.Get("Content-Type"), first.MediaType) {
		t.Fatalf("candidate file: st=%d ct=%q len=%d", st, fhdr.Get("Content-Type"), len(fb))
	}
}
