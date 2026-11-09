// HTTP contract tests of /v1 with httptest over the real adapters.
// Realise T-HUB-012, T-HUB-017, T-HUB-018 and T-HUB-019 (§3.6).
package http_v1_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/ging/alexandria/vhub/internal/server/domain"
	"github.com/ging/alexandria/vhub/internal/testkit"
)

// tokenOf maps a role to its fixture bearer token.
var tokenOf = map[domain.Role]string{domain.Editor: testkit.EditorToken, domain.Releaser: testkit.ReleaserToken, domain.Member: testkit.MemberToken}

// pathOf instantiates the route of an action with ids that do not exist.
func pathOf(a domain.Action) (string, string) {
	method, path, _ := strings.Cut(string(a), " ")
	path = strings.NewReplacer("{id}", "nope", "{version}", "9.9.9").Replace(path)
	return method, path
}

func TestT_HUB_017_EndpointRoleMatrix(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	for action, allowed := range domain.Matrix {
		for role, tok := range tokenOf {
			method, path := pathOf(action)
			st, _, b := h.Do(t, method, path, tok, []byte(`{}`), nil)
			ok := false
			for _, r := range allowed {
				ok = ok || r == role
			}
			forbidden := st == http.StatusForbidden && testkit.ProblemCode(b) == "API_FORBIDDEN_ROLE"
			if ok == forbidden {
				t.Errorf("%s as %s: status %d %s", action, role, st, b)
			}
		}
	}
}

func TestT_HUB_017_AuthenticationRequired(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	if st, _, b := h.Do(t, "GET", "/v1/versions", "", nil, nil); st != http.StatusUnauthorized || testkit.ProblemCode(b) != "API_FORBIDDEN_ROLE" {
		t.Fatalf("got %d %s", st, b)
	}
}

func TestT_HUB_018_ConcurrentWrites(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	_, hdr, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, testkit.DraftV1, nil)
	var d struct{ ID string }
	_ = json.Unmarshal(b, &d)
	etag := hdr.Get("ETag")
	if st, _, _ := h.Do(t, "PUT", "/v1/drafts/"+d.ID, testkit.EditorToken, testkit.DraftV1, map[string]string{"If-Match": etag}); st != http.StatusOK {
		t.Fatalf("first writer: %d", st)
	}
	if st, _, b := h.Do(t, "PUT", "/v1/drafts/"+d.ID, testkit.EditorToken, testkit.DraftV1, map[string]string{"If-Match": etag}); st != http.StatusPreconditionFailed || testkit.ProblemCode(b) != "API_PRECONDITION_FAILED" {
		t.Fatalf("second writer: %d %s", st, b)
	}
	if st, _, b := h.Do(t, "PUT", "/v1/drafts/"+d.ID, testkit.EditorToken, testkit.DraftV1, nil); st != http.StatusPreconditionRequired || testkit.ProblemCode(b) != "API_PRECONDITION_REQUIRED" {
		t.Fatalf("no If-Match: %d %s", st, b)
	}
}

func TestT_HUB_019_NotFoundOnEveryEndpoint(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	for action, roles := range domain.Matrix {
		if action == domain.DraftCreate || action == domain.EventsList {
			continue
		}
		method, path := pathOf(action)
		hdr := map[string]string{"If-Match": `"x"`}
		st, _, b := h.Do(t, method, path, tokenOf[roles[0]], []byte(`{"justification":"x"}`), hdr)
		if st != http.StatusNotFound || testkit.ProblemCode(b) != "API_NOT_FOUND" {
			t.Errorf("%s: %d %s", action, st, b)
		}
	}
}

func TestT_HUB_012_VersionsArePrivateNoCache(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	for _, p := range []string{"/v1/versions", "/v1/versions/1.0.0", "/v1/versions/2.0.0"} {
		st, hdr, _ := h.Do(t, "GET", p, testkit.MemberToken, nil, nil)
		if st != http.StatusOK || hdr.Get("Cache-Control") != "private, no-cache" {
			t.Errorf("%s: %d %q", p, st, hdr.Get("Cache-Control"))
		}
	}
}

func TestDraftLifecycleProblemsCarryPath(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	doc := testkit.DraftWith(testkit.DraftV1, `{"grace_window":"P0D","max_concurrent_versions":1}`)
	st, hdr, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, doc, nil)
	var p struct {
		Type, Code, Path string
		Findings         []struct{ Code string }
	}
	_ = json.Unmarshal(b, &p)
	if st != http.StatusUnprocessableEntity || hdr.Get("Content-Type") != "application/problem+json" || p.Type != "urn:eunomia:vhub:problem:LIFECYCLE_GRACE_ZERO" || p.Path == "" || len(p.Findings) != 2 {
		t.Fatalf("got %d %s", st, b)
	}
}
