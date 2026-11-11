//go:build !gui

// Tests of the default build: without the gui tag no assets exist and nothing is mounted on a listener.
// This guards that plain "go build" binaries expose only their APIs.
package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultBuildHasNoConsole(t *testing.T) {
	if Enabled() {
		t.Fatal("console assets present without the gui tag")
	}
}

func TestMountIsNoOpWithoutAssets(t *testing.T) {
	mux := http.NewServeMux()
	if Mount(mux, Config{Workspace: "hub"}) {
		t.Fatal("Mount reported a mounted console")
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/ui/", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status %d", rec.Code)
	}
}
