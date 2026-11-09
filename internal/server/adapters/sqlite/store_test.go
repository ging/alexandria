// Tests of the SQLite adapter: append-only tables refuse UPDATE and DELETE, and state survives reopening.
// Realise T-HUB-013 (REQ-HUB-013) and the persistence part of REQ-SYS-005.
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

func TestT_HUB_013_ImmutableTablesRefuseUpdateAndDelete(t *testing.T) {
	h := testkit.NewHTTPServer(t, "2026-09-01T10:00:00Z")
	h.PublishF6(t)
	h.Clock.Set("2026-11-10T00:00:00Z")
	if st, _, b := h.Do(t, "POST", "/v1/versions/1.0.0:revoke", testkit.ReleaserToken, []byte(`{"justification":"x"}`), nil); st != 200 {
		t.Fatalf("revoke: %d %s", st, b)
	}
	db := h.Store.DB()
	for _, table := range sqlite.ImmutableTables {
		if _, err := db.Exec("UPDATE " + table + " SET rowid = rowid"); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("UPDATE %s: %v", table, err)
		}
		if _, err := db.Exec("DELETE FROM " + table); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("DELETE %s: %v", table, err)
		}
	}
}

func TestT_HUB_013_ReleaseStoreHasNoMutators(t *testing.T) {
	rt := reflect.TypeOf((*app.ReleaseStore)(nil)).Elem()
	for i := 0; i < rt.NumMethod(); i++ {
		name := rt.Method(i).Name
		if strings.HasPrefix(name, "Update") || strings.HasPrefix(name, "Delete") || strings.HasPrefix(name, "Remove") {
			t.Errorf("ReleaseStore exposes %s", name)
		}
	}
}

func TestStateSurvivesReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hub.db")
	s, err := sqlite.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	d, _ := s.Drafts().Create([]byte(`{}`))
	s.Close()
	s, _ = sqlite.Open(path)
	defer s.Close()
	if got, err := s.Drafts().Get(d.ID); err != nil || got.ETag != d.ETag {
		t.Fatalf("got %+v %v", got, err)
	}
}
