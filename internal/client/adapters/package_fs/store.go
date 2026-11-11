// Package package_fs stores verified packages on disk, one directory per manifest digest with manifest.json and the files by path.
// Writes go through a temporary directory and a rename, so a package is stored whole or not at all (DP-2, REQ-CLI-007, REQ-CLI-017).
package package_fs

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
)

// Store implements app.PackageStore.
type Store struct{ dir string }

var _ app.PackageStore = Store{}

// Open creates the package directory.
func Open(dir string) (Store, error) { return Store{dir: dir}, os.MkdirAll(dir, 0o750) }

// path returns the directory of a digest.
func (s Store) path(d ni.Digest) string { return filepath.Join(s.dir, d.Segment()) }

// Get reads a package; ok is false when absent.
func (s Store) Get(d ni.Digest) (pkgv.Files, bool, error) {
	root := s.path(d)
	m, err := os.ReadFile(filepath.Join(root, "manifest.json")) // #nosec G304 -- digest-derived path.
	if errors.Is(err, os.ErrNotExist) {
		return pkgv.Files{}, false, nil
	}
	if err != nil {
		return pkgv.Files{}, false, err
	}
	f := pkgv.Files{Manifest: m, Content: map[string][]byte{}}
	files := filepath.Join(root, "files")
	err = filepath.WalkDir(files, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		b, err := os.ReadFile(p) // #nosec G304 -- inside the package directory.
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(files, p)
		f.Content[filepath.ToSlash(rel)] = b
		return nil
	})
	if errors.Is(err, os.ErrNotExist) {
		err = nil
	}
	return f, err == nil, err
}

// Put writes a package atomically.
func (s Store) Put(d ni.Digest, f pkgv.Files) error {
	tmp, err := os.MkdirTemp(s.dir, ".tmp-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmp) }()
	if err := os.WriteFile(filepath.Join(tmp, "manifest.json"), f.Manifest, 0o600); err != nil {
		return err
	}
	for p, b := range f.Content {
		dst := filepath.Join(tmp, "files", filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(dst, b, 0o600); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(s.path(d))
	return os.Rename(tmp, s.path(d))
}

// Delete removes a package.
func (s Store) Delete(d ni.Digest) error { return os.RemoveAll(s.path(d)) }

// List returns the stored digests.
func (s Store) List() ([]ni.Digest, error) {
	es, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	var out []ni.Digest
	for _, e := range es {
		if d, err := ni.FromSegment(e.Name()); err == nil && e.IsDir() {
			out = append(out, d)
		}
	}
	return out, nil
}
