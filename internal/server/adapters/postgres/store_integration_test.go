//go:build integration

// Live PostgreSQL contract tests for the server adapter: F6 lifecycle, immutability triggers, sequence uniqueness, and reopen persistence.
// Skips gracefully when PostgreSQL is unreachable; configure via ALEXANDRIA_TEST_POSTGRES_URL (DP-2, REQ-HUB-008, REQ-HUB-013).
package postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/server/adapters/postgres"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/testkit"
)

// defaultPostgresURL is the local Docker Compose PostgreSQL endpoint used when ALEXANDRIA_TEST_POSTGRES_URL is unset.
const defaultPostgresURL = "postgres://alexandria:alexandria@127.0.0.1:5432/alexandria_hub?sslmode=disable"

// testDSN returns the base PostgreSQL DSN from the environment or default.
func testDSN() string {
	if v := os.Getenv("ALEXANDRIA_TEST_POSTGRES_URL"); v != "" {
		return v
	}
	return defaultPostgresURL
}

// openIsolated opens a Store inside a fresh temporary PostgreSQL schema and drops it on test cleanup.
func openIsolated(t *testing.T) (*postgres.Store, string) {
	t.Helper()
	base := testDSN()
	adminDB, err := sql.Open("pgx", base)
	if err != nil {
		t.Skipf("postgres unavailable (%v)", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := adminDB.PingContext(ctx); err != nil {
		_ = adminDB.Close()
		t.Skipf("postgres not reachable (%v)", err)
	}
	raw := make([]byte, 6)
	_, _ = rand.Read(raw)
	schemaName := "test_srv_" + hex.EncodeToString(raw)
	if _, err := adminDB.ExecContext(context.Background(), "CREATE SCHEMA "+schemaName); err != nil {
		_ = adminDB.Close()
		t.Fatalf("create schema: %v", err)
	}
	u, err := url.Parse(base)
	if err != nil {
		_ = adminDB.Close()
		t.Fatal(err)
	}
	q := u.Query()
	q.Set("search_path", schemaName)
	u.RawQuery = q.Encode()
	scopedDSN := u.String()
	s, err := postgres.Open(scopedDSN)
	if err != nil {
		_, _ = adminDB.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
		_ = adminDB.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = s.Close()
		_, _ = adminDB.ExecContext(context.Background(), "DROP SCHEMA IF EXISTS "+schemaName+" CASCADE")
		_ = adminDB.Close()
	})
	return s, scopedDSN
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

// TestImmutableTablesRefuseUpdateAndDelete verifies BEFORE UPDATE and BEFORE DELETE triggers on append-only tables.
func TestImmutableTablesRefuseUpdateAndDelete(t *testing.T) {
	s, _ := openIsolated(t)
	f6On(t, s)
	for _, table := range postgres.ImmutableTables {
		col := "data"
		if table == "candidate_decisions" {
			col = "state"
		}
		if _, err := s.DB().Exec("UPDATE " + table + " SET " + col + " = " + col); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("UPDATE %s: %v", table, err)
		}
		if _, err := s.DB().Exec("DELETE FROM " + table); err == nil || !strings.Contains(err.Error(), "append-only") {
			t.Errorf("DELETE %s: %v", table, err)
		}
	}
}

// TestF6RunsOnPostgres executes the full F6 release and revocation workflow against PostgreSQL.
func TestF6RunsOnPostgres(t *testing.T) {
	s, _ := openIsolated(t)
	h := f6On(t, s)
	rels, err := s.Releases(testkit.NS)
	if err != nil || len(rels) != 2 || rels[0].Record.RevokedFrom == nil {
		t.Fatalf("got %d releases (err=%v), first revoked=%v", len(rels), err, len(rels) > 0 && rels[0].Record.RevokedFrom != nil)
	}
	if _, err := h.Hub.Listing.Latest(testkit.NS); err != nil {
		t.Fatal(err)
	}
}

// TestSequenceIsUniquePerNamespace verifies duplicate listing sequences return ErrSequenceTaken.
func TestSequenceIsUniquePerNamespace(t *testing.T) {
	s, _ := openIsolated(t)
	f6On(t, s)
	l, _, _ := s.LatestListing(testkit.NS)
	if err := s.PutListing(testkit.NS, l); !errors.Is(err, app.ErrSequenceTaken) {
		t.Fatalf("got %v, want ErrSequenceTaken", err)
	}
}

// TestStateSurvivesReopen checks draft persistence across store restarts on the same schema.
func TestStateSurvivesReopen(t *testing.T) {
	s, dsn := openIsolated(t)
	d, err := s.CreateDraft("demo", []byte(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s2, err := postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if got, err := s2.GetDraft("demo", d.ID); err != nil || got.ETag != d.ETag {
		t.Fatalf("got %+v %v", got, err)
	}
}
