// Content-addressed blob store on the local filesystem: <dir>/sha256/<hex>, written once and read-only.
// Implements the BlobStore port; bytes for a hash never change (DP-2, REQ-SYS-005, REQ-HUB-011).
package blob_fs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/server/app"
)

var hexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Store keeps blobs under a root directory.
type Store struct{ dir string }

var _ app.BlobStore = Store{}

// New creates the blob directory if needed.
func New(dir string) (Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "sha256"), 0o755); err != nil {
		return Store{}, err
	}
	return Store{dir: dir}, nil
}

// path returns the file path of a blob.
func (s Store) path(hex string) string { return filepath.Join(s.dir, "sha256", hex) }

// Put writes a blob once; an existing blob is left untouched.
func (s Store) Put(hex string, b []byte) error {
	if !hexRe.MatchString(hex) || ni.HexSum(b) != hex {
		return fmt.Errorf("blob_fs: %s is not the SHA-256 of the content", hex)
	}
	if _, err := os.Stat(s.path(hex)); err == nil {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Join(s.dir, "sha256"), ".tmp-*")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o444); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), s.path(hex))
}

// Get reads a blob.
func (s Store) Get(hex string) ([]byte, error) {
	if !hexRe.MatchString(hex) {
		return nil, app.ErrNotFound
	}
	b, err := os.ReadFile(s.path(hex))
	if errors.Is(err, os.ErrNotExist) {
		return nil, app.ErrNotFound
	}
	return b, err
}
