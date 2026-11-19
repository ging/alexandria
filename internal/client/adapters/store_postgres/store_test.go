// Unit tests for PostgreSQL DSN detection and constructor validation in the client store adapter.
// Live database contract tests live in store_integration_test.go behind the integration build tag (DP-2).
package store_postgres_test

import (
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/store_postgres"
)

// dsnCases lists valid and invalid PostgreSQL connection string inputs.
var dsnCases = map[string]bool{
	"postgres://alexandria:secret@localhost:5432/alexandria_client":   true,
	"postgresql://alexandria:secret@localhost:5432/alexandria_client": true,
	"file:client.db":  false,
	"state/client.db": false,
	"":                false,
}

// TestIsDSN checks recognition of postgres:// and postgresql:// prefixes.
func TestIsDSN(t *testing.T) {
	for in, want := range dsnCases {
		if got := store_postgres.IsDSN(in); got != want {
			t.Errorf("IsDSN(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestOpenRejectsNonPostgresTarget verifies Open rejects local file paths before dialing.
func TestOpenRejectsNonPostgresTarget(t *testing.T) {
	if _, err := store_postgres.Open("state/client.db"); err == nil {
		t.Fatal("expected non-postgres DSN to be rejected")
	}
}
