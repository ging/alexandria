// Integration test: the real client adapters (http_hub, did_resolver, store_sqlite, package_fs) sync against a real hub over HTTP.
// Covers the M5 exit criterion and the HTTP side of S1 (T-CLI-003, T-CLI-004, T-CLI-005).
package http_hub_test

import (
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/did_resolver"
	"github.com/ging/alexandria/internal/client/adapters/http_hub"
	"github.com/ging/alexandria/internal/client/adapters/package_fs"
	"github.com/ging/alexandria/internal/client/adapters/store_sqlite"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/testkit"
)

// syncer builds a Sync over real adapters for hub h and namespace ns, with state in dir.
func syncer(t *testing.T, h *testkit.HTTPServer, ns, dir string) *app.Sync {
	t.Helper()
	ev := &testkit.Recorder{}
	store, err := store_sqlite.Open(filepath.Join(dir, "client.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	pkgs, _ := package_fs.Open(filepath.Join(dir, "packages"))
	hub, _ := http_hub.New(h.URL, ns, "")
	cell := &app.Cell{}
	iri := testkit.Base + "/v1/namespaces/" + ns
	trust := app.NewHubTrust(h.DID, did_resolver.New(false), h.Clock, 6*timex.Hour, ev, iri)
	ag := &app.Agreements{Repo: store_sqlite.Agreements{S: store}, Events: ev, Clock: h.Clock, Namespace: iri}
	st := &app.Status{Cell: cell, Rules: &app.RulesCell{}, Clock: h.Clock, Trust: trust, Packages: pkgs, Events: ev, Namespace: iri}
	return &app.Sync{
		Hub: hub, Trust: trust, Continuity: store, Packages: pkgs, Agreements: ag, Audit: &app.Audit{Window: 10, Events: ev, Clock: h.Clock},
		Status: st, Cell: cell, Clock: h.Clock, Events: ev, Namespace: iri, HubDID: h.DID, Context: testkit.Context,
	}
}

// hub publishes F6 behind HTTP.
func hub(t *testing.T) *testkit.HTTPServer {
	t.Helper()
	h := testkit.NewHTTPServer(t, "2026-08-01T00:00:00Z")
	h.Setup(t)
	h.PublishF6(t)
	h.Clock.Set("2026-10-10T12:00:00Z")
	return h
}

func TestT_CLI_003_SyncOverHTTPAgainstRealHub(t *testing.T) {
	s := syncer(t, hub(t), "demo", t.TempDir())
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if snap := s.Cell.Load(); snap == nil || len(snap.Records) != 2 || len(snap.Packages) != 2 {
		t.Fatalf("snapshot %+v", snap)
	}
}

func TestT_CLI_004_NamespaceNotFoundOverHTTP(t *testing.T) {
	if err := syncer(t, hub(t), "nope", t.TempDir()).Run(); err == nil || err.Code != codes.SyncNamespaceNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_005_PersistedStateSurvivesNewProcess(t *testing.T) {
	h, dir := hub(t), t.TempDir()
	if err := syncer(t, h, "demo", dir).Run(); err != nil {
		t.Fatal(err)
	}
	s := syncer(t, h, "demo", dir)
	if err := s.Restore(); err != nil || s.Cell.Load() == nil || len(s.Cell.Load().Packages) != 2 {
		t.Fatalf("restore: %v", err)
	}
}
