// Tests of the console handler: configuration injection, application-route fallback, asset caching and missing files.
// They run against an in-memory build so they do not depend on the gui build tag.
package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// build is a minimal console build: the page and one hashed asset.
var build = fstest.MapFS{
	"index.html":        {Data: []byte("<html><head><title>x</title></head><body></body></html>")},
	"assets/app-abc.js": {Data: []byte("console.log(1)")},
	"favicon.svg":       {Data: []byte("<svg/>")},
}

// get performs a GET against a handler serving build with the hub workspace.
func get(t *testing.T, target string, cfg Config) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	New(build, cfg).ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil))
	return rec
}

func TestIndexCarriesInjectedConfiguration(t *testing.T) {
	rec := get(t, "/ui/", Config{Workspace: "hub", Issuer: "https://idp.example"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `window.__ALEXANDRIA__={"workspace":"hub","issuer":"https://idp.example"};`) {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestIndexIsNotCached(t *testing.T) {
	if cc := get(t, "/ui/", Config{}).Header().Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("Cache-Control = %q", cc)
	}
}

func TestApplicationRouteFallsBackToIndex(t *testing.T) {
	rec := get(t, "/ui/hub/n/demo/versions", Config{Workspace: "hub"})
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "<title>x</title>") {
		t.Fatalf("status %d body %s", rec.Code, rec.Body)
	}
}

func TestHashedAssetIsImmutable(t *testing.T) {
	rec := get(t, "/ui/assets/app-abc.js", Config{})
	if rec.Code != http.StatusOK || rec.Body.String() != "console.log(1)" || !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestRootFileIsServedWithoutImmutableCaching(t *testing.T) {
	rec := get(t, "/ui/favicon.svg", Config{})
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "" {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
}

func TestMissingAssetIsNotFound(t *testing.T) {
	if rec := get(t, "/ui/assets/gone.js", Config{}); rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestConfigurationCannotCloseTheScriptElement(t *testing.T) {
	body := get(t, "/ui/", Config{Issuer: "</script><b>"}).Body.String()
	if strings.Contains(body, "</script><b>") {
		t.Fatalf("unescaped configuration in %s", body)
	}
}
