// Integration test: the real client adapters (http_hub, did_web, store_fs) sync against a real hub served over HTTP.
// Covers the M5 exit criterion and the HTTP side of T-CLI-003 and T-CLI-027 (REQ-CLI-003, REQ-CLI-027).
package http_hub_test

import (
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/did_web"
	"github.com/ging/alexandria/internal/client/adapters/http_hub"
	"github.com/ging/alexandria/internal/client/adapters/receipt_scitt_fake"
	"github.com/ging/alexandria/internal/client/adapters/store_fs"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/testkit"
)

// syncer builds a Sync over real adapters for hub h with state in dir.
func syncer(h *testkit.HTTPServer, dir, token string) (*app.Sync, *testkit.Recorder) {
	ev := &testkit.Recorder{}
	store, _ := store_fs.Open(dir)
	cell := &app.Cell{}
	trust := app.NewHubTrust(h.DID, did_web.New(true), h.Clock, 6*timex.Hour, ev, testkit.Space)
	ag := &app.Agreements{Repo: store.Agreements(), Events: ev, Clock: h.Clock, Space: testkit.Space}
	st := &app.Status{Cell: cell, Clock: h.Clock, Trust: trust, Packages: store, Events: ev, Space: testkit.Space}
	return &app.Sync{
		Hub: http_hub.New(h.URL, token), Trust: trust, Receipts: receipt_scitt_fake.Verifier{}, Continuity: store, Packages: store,
		Agreements: ag, Audit: &app.Audit{Window: 10, Events: ev, Clock: h.Clock}, Status: st, Cell: cell, Clock: h.Clock, Events: ev, Space: testkit.Space,
	}, ev
}

func TestT_CLI_003_SyncOverHTTPAgainstRealHub(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	h.Clock.Set("2026-10-10T12:00:00Z")
	s, _ := syncer(h, t.TempDir(), testkit.MemberToken)
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	if snap := s.Cell.Load(); snap == nil || len(snap.Records) != 2 || len(snap.Packages) != 2 {
		t.Fatalf("snapshot %+v", snap)
	}
}

func TestT_CLI_004_UnauthorizedOverHTTP(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	s, _ := syncer(h, t.TempDir(), "wrong-token")
	if err := s.Run(); err == nil || err.Code != "SYNC_UNAUTHORIZED" {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_005_FileStoreRestoresAfterRestart(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	h.Clock.Set("2026-10-10T12:00:00Z")
	dir := t.TempDir()
	s, _ := syncer(h, dir, testkit.MemberToken)
	if err := s.Run(); err != nil {
		t.Fatal(err)
	}
	again, _ := syncer(h, dir, testkit.MemberToken)
	if err := again.Restore(); err != nil || again.Cell.Load() == nil || len(again.Cell.Load().Packages) != 2 {
		t.Fatalf("restore: %v", err)
	}
	if doc, _ := again.Trust.Cache(); doc == nil {
		t.Fatal("DID document not persisted")
	}
}
