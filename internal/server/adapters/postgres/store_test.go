// Unit tests for PostgreSQL DSN detection and constructor validation in the server adapter.
// Live database contract tests live in store_integration_test.go behind the integration build tag (DP-2).
package postgres_test

import (
	"testing"

	"github.com/ging/alexandria/internal/server/adapters/postgres"
)

// dsnCases lists valid and invalid PostgreSQL connection string inputs.
var dsnCases = map[string]bool{
	"postgres://alexandria:secret@localhost:5432/alexandria":   true,
	"postgresql://alexandria:secret@localhost:5432/alexandria": true,
	"file:../var/hub/hub.db":                                   false,
	"/var/lib/alexandria/hub.db":                               false,
	"":                                                         false,
}

// TestIsDSN checks recognition of postgres:// and postgresql:// prefixes.
func TestIsDSN(t *testing.T) {
	for in, want := range dsnCases {
		if got := postgres.IsDSN(in); got != want {
			t.Errorf("IsDSN(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestOpenRejectsNonPostgresTarget verifies Open rejects SQLite paths before dialing.
func TestOpenRejectsNonPostgresTarget(t *testing.T) {
	if _, err := postgres.Open("file:hub.db"); err == nil {
		t.Fatal("expected non-postgres DSN to be rejected")
	}
}
