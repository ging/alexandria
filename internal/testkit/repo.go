// Package testkit locates repository files from any test package by walking up to go.mod.
// Used to load the Annex C examples and the F6 drafts (F8 §8.7).
package testkit

import (
	"os"
	"path/filepath"
	"testing"
)

// repoFile returns the absolute path of a file relative to the repository root.
func repoFile(t *testing.T, rel string) string {
	t.Helper()
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, rel)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found")
		}
		dir = parent
	}
}

// RepoFile is the exported form of repoFile.
func RepoFile(t *testing.T, rel string) string { t.Helper(); return repoFile(t, rel) }
