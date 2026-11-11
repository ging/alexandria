// Unit tests for the alexandria-server composition root and laboratory TLS certificate bootstrapper.
// Verifies idempotent generation, missing-file recovery, and valid X.509 keypair output (DP-14).
package main

import (
	"bytes"
	"crypto/tls"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// fixedNow is the deterministic reference instant used for test certificate validity windows.
var fixedNow = time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)

// tlsPaths returns certPath and keyPath inside a fresh temporary directory.
func tlsPaths(t *testing.T) (string, string) {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "tls")
	return filepath.Join(dir, "hub.crt"), filepath.Join(dir, "hub.key")
}

// TestEnsureLabTLSGeneratesValidPair verifies that ensureLabTLS creates a loadable X.509 certificate and key when absent.
func TestEnsureLabTLSGeneratesValidPair(t *testing.T) {
	certPath, keyPath := tlsPaths(t)
	if err := ensureLabTLS(certPath, keyPath, "https://hub.example.local:8443", fixedNow); err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		t.Fatalf("generated pair is not loadable: %v", err)
	}
}

// TestEnsureLabTLSPreservesExistingPair verifies that ensureLabTLS leaves an existing certificate and key untouched.
func TestEnsureLabTLSPreservesExistingPair(t *testing.T) {
	certPath, keyPath := tlsPaths(t)
	if err := ensureLabTLS(certPath, keyPath, "https://127.0.0.1:8443", fixedNow); err != nil {
		t.Fatal(err)
	}
	origCert, _ := os.ReadFile(certPath)
	origKey, _ := os.ReadFile(keyPath)
	if err := ensureLabTLS(certPath, keyPath, "https://127.0.0.1:8443", fixedNow); err != nil {
		t.Fatal(err)
	}
	afterCert, _ := os.ReadFile(certPath)
	afterKey, _ := os.ReadFile(keyPath)
	if !bytes.Equal(origCert, afterCert) || !bytes.Equal(origKey, afterKey) {
		t.Fatal("existing TLS pair was overwritten")
	}
}

// TestEnsureLabTLSRegeneratesWhenOneFileMissing verifies recovery when hub.key exists without hub.crt.
func TestEnsureLabTLSRegeneratesWhenOneFileMissing(t *testing.T) {
	certPath, keyPath := tlsPaths(t)
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, []byte("orphan"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := ensureLabTLS(certPath, keyPath, "https://127.0.0.1:8443", fixedNow); err != nil {
		t.Fatal(err)
	}
	if _, err := tls.LoadX509KeyPair(certPath, keyPath); err != nil {
		t.Fatalf("regenerated pair is not loadable: %v", err)
	}
}

// TestEnsureLabTLSRejectsHalfConfiguredPaths verifies that missing either certPath or keyPath returns an error.
func TestEnsureLabTLSRejectsHalfConfiguredPaths(t *testing.T) {
	certPath, _ := tlsPaths(t)
	if err := ensureLabTLS(certPath, "", "https://127.0.0.1:8443", fixedNow); err == nil {
		t.Fatal("expected error when keyPath is empty")
	}
}
