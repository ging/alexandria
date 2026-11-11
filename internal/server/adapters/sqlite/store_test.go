// Contract tests of the SQLite adapter: the F6 use cases run on it, immutable tables refuse UPDATE and DELETE, and state survives reopening.
// Fixtures: a temporary database driven by the testkit hub (T-HUB-013, T-HUB-008, DP-2).
package sqlite_test

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/server/adapters/sqlite"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// open returns a fresh store in a temporary directory.
func open(t *testing.T) (*sqlite.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hub.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, path
}

// f6On publishes the F6 releases and revokes 1.0.0 on store.
func f6On(t *testing.T, store app.Store) *testkit.Server {
	t.Helper()
	h := testkit.NewServerOn(t, "2026-08-01T00:00:00Z", store)
	h.PublishF6(t)
	if _, err := h.Hub.Vocabularies.Create(testkit.Sergio, testkit.NS, app.VocabInput{Name: "ctx", Kind: "proxy", MediaType: "application/ld+json", Upstream: "https://x.example/c"}); err != nil {
		t.Fatal(err)
	}
	h.Clock.Set("2026-11-10T00:00:00Z")
	if _, err := h.Hub.Revocation.Revoke(testkit.Sergio, testkit.NS, "1.0.0", app.RevokeOptions{Justification: "x"}); err != nil {
		t.Fatal(err)
	}
	return h
}

func TestT_HUB_013_ImmutableTablesRefuseUpdateAndDelete(t *testing.T) {
	s, _ := open(t)
	f6On(t, s)
	for _, table := range sqlite.ImmutableTables {
		if _, err := s.DB().Exec("UPDATE " + table + " SET rowid = rowid"); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("UPDATE %s: %v", table, err)
		}
		if _, err := s.DB().Exec("DELETE FROM " + table); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("DELETE %s: %v", table, err)
		}
	}
}

func TestT_HUB_013_ReleaseStoreHasNoMutators(t *testing.T) {
	rt := reflect.TypeOf((*app.ReleaseStore)(nil)).Elem()
	for i := 0; i < rt.NumMethod(); i++ {
		if name := rt.Method(i).Name; strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete") || strings.HasPrefix(name, "Remove") {
			t.Errorf("ReleaseStore exposes %s", name)
		}
	}
}

func TestF6RunsOnSQLite(t *testing.T) {
	s, _ := open(t)
	h := f6On(t, s)
	rels, _ := s.Releases(testkit.NS)
	if len(rels) != 2 || rels[0].Record.RevokedFrom == nil {
		t.Fatalf("got %d releases, first revoked=%v", len(rels), rels[0].Record.RevokedFrom != nil)
	}
	if _, err := h.Hub.Listing.Latest(testkit.NS); err != nil {
		t.Fatal(err)
	}
}

func TestT_HUB_008_SequenceIsUniquePerNamespace(t *testing.T) {
	s, _ := open(t)
	f6On(t, s)
	l, _, _ := s.LatestListing(testkit.NS)
	if err := s.PutListing(testkit.NS, l); err != app.ErrSequenceTaken {
		t.Fatalf("got %v", err)
	}
}

func TestStateSurvivesReopen(t *testing.T) {
	s, path := open(t)
	d, _ := s.CreateDraft("demo", []byte(`{}`))
	_ = s.Close()
	s2, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if got, err := s2.GetDraft("demo", d.ID); err != nil || got.ETag != d.ETag {
		t.Fatalf("got %+v %v", got, err)
	}
}
