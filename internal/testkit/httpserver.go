// Package testkit provides an in-process alexandria-server over the real adapters (SQLite, blob_fs, signer_file, auth_static) behind httptest.
// Used by HTTP contract tests and by client tests that need a real hub over HTTP (F8 §8.7).
package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/server/adapters/auth_static"
	"github.com/ging/alexandria/internal/server/adapters/blob_fs"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/adapters/http_did"
	"github.com/ging/alexandria/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/internal/server/adapters/http_v1"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/adapters/scitt_fake"
	"github.com/ging/alexandria/internal/server/adapters/signer_file"
	"github.com/ging/alexandria/internal/server/adapters/sqlite"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Bearer tokens of the HTTP fixture, one per role.
const (
	EditorToken   = "editor-token"
	ReleaserToken = "releaser-token"
	MemberToken   = "member-token"
)

// HTTPServer is a hub served over HTTP with real persistence.
type HTTPServer struct {
	URL   string
	DID   did.DID
	Clock *FakeClock
	Store *sqlite.Store
	Hub   *app.Hub
	srv   *httptest.Server
}

// NewHTTPServer starts a hub on a random port with its state in a temp dir.
func NewHTTPServer(t *testing.T, t0 string) *HTTPServer {
	t.Helper()
	dir := t.TempDir()
	clk := NewClock(t0)
	h := &HTTPServer{Clock: clk}
	mux := http.NewServeMux()
	h.srv = httptest.NewServer(httpx.WithDate(clk, mux))
	t.Cleanup(h.srv.Close)
	h.URL = h.srv.URL
	u, _ := url.Parse(h.URL)
	h.DID = did.DID("did:web:" + strings.ReplaceAll(u.Host, ":", "%3A"))
	store, err := sqlite.Open(filepath.Join(dir, "hub.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	blobs, _ := blob_fs.New(filepath.Join(dir, "blobs"))
	signer, err := signer_file.Load(filepath.Join(dir, "hub.key"), h.DID)
	if err != nil {
		t.Fatal(err)
	}
	h.Store = store
	h.Hub = app.New(app.Deps{
		Drafts: store.Drafts(), Candidates: store.Candidates(), Releases: store.Releases(), Listings: store.Listings(),
		Blobs: blobs, Log: scitt_fake.Log{Signer: signer, Clock: clk}, Signer: signer,
		Compiler: compiler_poc.Compiler{Defaults: lifecycle.Defaults()}, Events: store.Events(), Clock: clk, DID: h.DID,
		Defaults: lifecycle.Defaults(),
	})
	tokens := auth_static.Tokens{
		EditorToken: Editor, ReleaserToken: Releaser, MemberToken: Member,
		"all-token": {Name: "admin", Roles: []domain.Role{domain.Editor, domain.Releaser, domain.Member}},
	}
	http_v1.Handler{Hub: h.Hub, Auth: tokens}.Register(mux)
	http_pub.Handler{Pub: h.Hub.Publication}.Register(mux)
	http_did.Handler{Listing: h.Hub.Listing}.Register(mux)
	return h
}

// Do sends a request with an optional bearer token and body, returning status, headers and body.
func (h *HTTPServer) Do(t *testing.T, method, path, token string, body []byte, hdr map[string]string) (int, http.Header, []byte) {
	t.Helper()
	req, _ := http.NewRequestWithContext(context.Background(), method, h.URL+path, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	cl := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := cl.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, resp.Header, b
}

// Publish creates, proposes and approves a draft over HTTP at instant at.
func (h *HTTPServer) Publish(t *testing.T, at string, doc []byte, approve string) []byte {
	t.Helper()
	h.Clock.Set(at)
	st, _, b := h.Do(t, "POST", "/v1/drafts", EditorToken, doc, nil)
	if st != http.StatusCreated {
		t.Fatalf("create draft: %d %s", st, b)
	}
	var d struct{ ID string }
	_ = json.Unmarshal(b, &d)
	st, _, b = h.Do(t, "POST", "/v1/drafts/"+d.ID+":propose", EditorToken, nil, nil)
	if st != http.StatusCreated {
		t.Fatalf("propose: %d %s", st, b)
	}
	var c struct{ ID string }
	_ = json.Unmarshal(b, &c)
	st, _, b = h.Do(t, "POST", "/v1/candidates/"+c.ID+":approve", ReleaserToken, []byte(approve), nil)
	if st != http.StatusCreated {
		t.Fatalf("approve: %d %s", st, b)
	}
	return b
}

// PublishF6 publishes 1.0.0 and 2.0.0 at the F6 instants.
func (h *HTTPServer) PublishF6(t *testing.T) {
	t.Helper()
	h.Publish(t, "2026-09-01T10:00:00Z", DraftV1, "")
	h.Publish(t, "2026-10-01T10:00:00Z", DraftV2, "")
}

// ProblemCode extracts the code of a problem body.
func ProblemCode(b []byte) string {
	var p struct{ Code string }
	_ = json.Unmarshal(b, &p)
	return p.Code
}
