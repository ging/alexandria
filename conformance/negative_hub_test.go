// Negative battery for the hub: one N-<CODE> case per code emitted by C-HUB, over the real HTTP adapters.
// Drafts come from conformance/corpus/drafts; each case expects exactly its code (F5 §5.2, check D3).
package conformance

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// hubCase is one negative case run against a hub served over HTTP.
type hubCase struct {
	code string
	run  func(t *testing.T, h *testkit.HTTPServer) (int, []byte)
}

// candidate creates and proposes a draft and returns the candidate id.
func candidate(t *testing.T, h *testkit.HTTPServer, doc []byte) string {
	t.Helper()
	st, _, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, doc, nil)
	if st != 201 {
		t.Fatalf("draft: %d %s", st, b)
	}
	id := jsonField(b, "id")
	st, _, b = h.Do(t, "POST", "/v1/drafts/"+id+":propose", testkit.EditorToken, nil, nil)
	if st != 201 {
		t.Fatalf("propose: %d %s", st, b)
	}
	return jsonField(b, "id")
}

// approve proposes a draft at ts and approves it with body.
func approve(t *testing.T, h *testkit.HTTPServer, ts string, doc []byte, body string) (int, []byte) {
	t.Helper()
	h.Clock.Set(ts)
	st, _, b := h.Do(t, "POST", "/v1/candidates/"+candidate(t, h, doc)+":approve", testkit.ReleaserToken, []byte(body), nil)
	return st, b
}

// revoke revokes a version at ts with body.
func revoke(t *testing.T, h *testkit.HTTPServer, ts, v, body string) (int, []byte) {
	t.Helper()
	h.Clock.Set(ts)
	st, _, b := h.Do(t, "POST", "/v1/versions/"+v+":revoke", testkit.ReleaserToken, []byte(body), nil)
	return st, b
}

// v1Only publishes 1.0.0 only.
func v1Only(t *testing.T, h *testkit.HTTPServer) {
	h.Publish(t, "2026-09-01T10:00:00Z", testkit.DraftV1, "")
}

// hubCases lists the hub negative cases of F5.
var hubCases = []hubCase{
	{"API_FORBIDDEN_ROLE", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		st, _, b := h.Do(t, "POST", "/v1/candidates/"+candidate(t, h, testkit.DraftV1)+":approve", testkit.EditorToken, nil, nil)
		return st, b
	}},
	{"API_PRECONDITION_REQUIRED", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		_, _, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, testkit.DraftV1, nil)
		st, _, b := h.Do(t, "PUT", "/v1/drafts/"+jsonField(b, "id"), testkit.EditorToken, testkit.DraftV1, nil)
		return st, b
	}},
	{"API_PRECONDITION_FAILED", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		_, hdr, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, testkit.DraftV1, nil)
		id, etag := jsonField(b, "id"), hdr.Get("ETag")
		h.Do(t, "PUT", "/v1/drafts/"+id, testkit.EditorToken, testkit.DraftV1, map[string]string{"If-Match": etag})
		st, _, b := h.Do(t, "PUT", "/v1/drafts/"+id, testkit.EditorToken, testkit.DraftV1, map[string]string{"If-Match": etag})
		return st, b
	}},
	{"API_NOT_FOUND", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		st, _, b := h.Do(t, "GET", "/v1/drafts/inexistente", testkit.EditorToken, nil, nil)
		return st, b
	}},
	{"API_INVALID_STATE", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		id := candidate(t, h, testkit.DraftV1)
		h.Do(t, "POST", "/v1/candidates/"+id+":approve", testkit.ReleaserToken, nil, nil)
		st, _, b := h.Do(t, "POST", "/v1/candidates/"+id+":approve", testkit.ReleaserToken, nil, nil)
		return st, b
	}},
	{"DATES_GRACE_ON_COMPATIBLE", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return approve(t, h, "2026-09-10T10:00:00Z", testkit.Draft("1.1.0", "odrl:use"), `{"grace_window":"P10D"}`)
	}},
	{"DATES_EMERGENCY_ON_COMPATIBLE", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return approve(t, h, "2026-09-10T10:00:00Z", testkit.Draft("1.1.0", "odrl:use"), `{"emergency":true}`)
	}},
	{"DATES_ZERO_GRACE_WITHOUT_EMERGENCY", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return approve(t, h, "2026-10-01T10:00:00Z", testkit.DraftV2, `{"grace_window":"P0D"}`)
	}},
	{"DATES_EMERGENCY_WITH_GRACE", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return approve(t, h, "2026-10-01T10:00:00Z", testkit.DraftV2, `{"emergency":true,"grace_window":"P10D","justification":"x"}`)
	}},
	{"DATES_JUSTIFICATION_REQUIRED", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return approve(t, h, "2026-10-01T10:00:00Z", testkit.DraftV2, `{"emergency":true}`)
	}},
	{"DATES_NON_MONOTONIC", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		h.PublishF6(t)
		return approve(t, h, "2026-10-05T10:00:00Z", testkit.Draft("3.0.0", "odrl:use"), `{"grace_window":"P5D"}`)
	}},
	{"DATES_TOO_MANY_VERSIONS", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		h.PublishF6(t)
		return approve(t, h, "2026-10-05T10:00:00Z", testkit.DraftWith(testkit.Draft("2.1.0", "odrl:use"), `{"max_concurrent_versions":2}`), `{}`)
	}},
	{"DATES_REVOCATION_TOO_EARLY", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		h.PublishF6(t)
		return revoke(t, h, "2026-11-10T00:00:00Z", "1.0.0", `{"justification":"x","revoked_from":"2026-11-11T00:00:00Z"}`)
	}},
	{"DATES_REVOKE_LEAVES_NO_HEAD", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		v1Only(t, h)
		return revoke(t, h, "2026-09-10T00:00:00Z", "1.0.0", `{"justification":"x"}`)
	}},
	{"DATES_ALREADY_REVOKED", func(t *testing.T, h *testkit.HTTPServer) (int, []byte) {
		h.PublishF6(t)
		revoke(t, h, "2026-11-10T00:00:00Z", "1.0.0", `{"justification":"x"}`)
		return revoke(t, h, "2026-11-10T00:00:01Z", "1.0.0", `{"justification":"x"}`)
	}},
}

func TestN_HubCodes(t *testing.T) {
	for _, c := range hubCases {
		t.Run("N-"+c.code, func(t *testing.T) {
			h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
			st, b := c.run(t, h)
			if st < 400 || testkit.ProblemCode(b) != c.code {
				t.Fatalf("got %d %s", st, b)
			}
		})
	}
}

func TestN_LifecycleCorpus(t *testing.T) {
	files, _ := filepath.Glob(filepath.Join("corpus", "drafts", "LIFECYCLE_*.json"))
	if len(files) != 7 {
		t.Fatalf("%d LIFECYCLE documents, want 7", len(files))
	}
	for _, f := range files {
		code := trimExt(filepath.Base(f))
		t.Run("N-"+code, func(t *testing.T) {
			doc, _ := os.ReadFile(f)
			h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
			st, _, b := h.Do(t, "POST", "/v1/drafts", testkit.EditorToken, doc, nil)
			if st != http.StatusUnprocessableEntity || testkit.ProblemCode(b) != code || jsonField(b, "findings") != "" {
				t.Fatalf("got %d %s", st, b)
			}
		})
	}
}

func TestN_RELEASE_ABORTED(t *testing.T) {
	t.Run("N-RELEASE_ABORTED", func(t *testing.T) {
		s := testkit.NewServer("2026-09-01T10:00:00Z")
		id := s.Propose(t, testkit.DraftV1)
		s.DB.Faults.Register = errors.New("transparency log does not answer")
		_, err := s.Hub.Release.Approve(testkit.Releaser, id, app.ApproveOptions{})
		if codeOfErr(err) != "RELEASE_ABORTED" {
			t.Fatalf("got %v", err)
		}
	})
}
