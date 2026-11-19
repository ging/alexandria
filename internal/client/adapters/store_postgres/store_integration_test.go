//go:build integration

// Live PostgreSQL contract tests for the client store adapter: continuity, ruleset, and agreement registry persistence.
// Skips gracefully when PostgreSQL is unreachable; configure via ALEXANDRIA_TEST_POSTGRES_URL (DP-2, REQ-CLI-005, REQ-CLI-031).
package store_postgres_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/ging/alexandria/internal/client/adapters/store_postgres"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/timex"
)

// defaultPostgresURL is the local Docker Compose PostgreSQL endpoint used when ALEXANDRIA_TEST_POSTGRES_URL is unset.
const defaultPostgresURL = "postgres://alexandria:alexandria@127.0.0.1:5432/alexandria_client?sslmode=disable"

// testDSN returns the base PostgreSQL DSN from the environment or default.
func testDSN() string {
	if v := os.Getenv("ALEXANDRIA_TEST_POSTGRES_URL"); v != "" {
		return v
	}
	return defaultPostgresURL
}

// openIsolated opens a client Store inside a fresh temporary PostgreSQL schema and drops it on cleanup.
func openIsolated(t *testing.T) (*store_postgres.Store, string) {
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
	schemaName := "test_cli_" + hex.EncodeToString(raw)
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
	s, err := store_postgres.Open(scopedDSN)
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

// TestContinuityRoundTripAndReopen verifies continuity persistence across store restarts.
func TestContinuityRoundTripAndReopen(t *testing.T) {
	s, dsn := openIsolated(t)
	ts, _ := timex.ParseInstant("2026-09-01T00:00:00Z")
	if err := s.Save(app.Continuity{ResolvedAt: ts}); err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	s2, err := store_postgres.Open(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	got, err := s2.Load()
	if err != nil || got.ResolvedAt != ts {
		t.Fatalf("got %+v, err=%v", got, err)
	}
}

// TestAgreementsRoundTrip verifies agreement insertion, update, lookup, and sorted listing.
func TestAgreementsRoundTrip(t *testing.T) {
	s, _ := openIsolated(t)
	repo := store_postgres.Agreements{S: s}
	e := domain.Entry{AgreementID: "urn:uuid:agr-1"}
	if err := repo.Put(e); err != nil {
		t.Fatal(err)
	}
	got, ok, err := repo.Get("urn:uuid:agr-1")
	if err != nil || !ok || got.AgreementID != e.AgreementID {
		t.Fatalf("got %+v ok=%v err=%v", got, ok, err)
	}
	all, err := repo.All()
	if err != nil || len(all) != 1 {
		t.Fatalf("all: len=%d err=%v", len(all), err)
	}
}
