// Filesystem stores of the client: continuity (sequence, known dates, DID cache), verified packages and agreements.
// Writes are atomic renames so a restart never sees half a state (REQ-SYS-006, REQ-CLI-005, REQ-CLI-017).
package store_fs

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
)

// Store keeps client state under one directory.
type Store struct {
	dir string
	mu  sync.Mutex
}

var (
	_ app.ContinuityStore = (*Store)(nil)
	_ app.PackageStore    = (*Store)(nil)
	_ app.AgreementRepo   = agreementRepo{}
)

// Open creates the state directory.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(filepath.Join(dir, "packages"), 0o755); err != nil {
		return nil, err
	}
	return &Store{dir: dir}, nil
}

// writeAtomic writes a file through a temporary sibling and a rename.
func writeAtomic(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// Load reads continuity.json.
func (s *Store) Load() (app.Continuity, error) {
	var c app.Continuity
	b, err := os.ReadFile(filepath.Join(s.dir, "continuity.json"))
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// Save writes continuity.json.
func (s *Store) Save(c app.Continuity) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return writeAtomic(filepath.Join(s.dir, "continuity.json"), b)
}

// pkgDir is the directory of a package.
func (s *Store) pkgDir(d ni.Digest) string { return filepath.Join(s.dir, "packages", d.Segment()) }

// Get reads a stored package, including any file on disk under files/.
func (s *Store) Get(d ni.Digest) (pkgv.Files, bool, error) {
	dir := s.pkgDir(d)
	var f pkgv.Files
	for name, dst := range map[string]*[]byte{"manifest.json": &f.Manifest, "manifest.sig": &f.Sig, "manifest.receipt": &f.Receipt, "space-lock.json": &f.SpaceLock} {
		b, err := os.ReadFile(filepath.Join(dir, name))
		if errors.Is(err, os.ErrNotExist) {
			return pkgv.Files{}, false, nil
		}
		if err != nil {
			return pkgv.Files{}, false, err
		}
		*dst = b
	}
	f.Content = map[string][]byte{}
	root := filepath.Join(dir, "files")
	err := filepath.WalkDir(root, func(p string, e fs.DirEntry, err error) error {
		if err != nil || e.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(root, p)
		b, err := os.ReadFile(p)
		f.Content[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return pkgv.Files{}, false, err
	}
	return f, true, nil
}

// Put writes a verified package; the directory appears complete through a rename.
func (s *Store) Put(d ni.Digest, f pkgv.Files) error {
	final := s.pkgDir(d)
	tmp := final + ".tmp"
	_ = os.RemoveAll(tmp)
	files := map[string][]byte{"manifest.json": f.Manifest, "manifest.sig": f.Sig, "manifest.receipt": f.Receipt, "space-lock.json": f.SpaceLock}
	for p, b := range f.Content {
		if strings.Contains(p, "..") {
			continue
		}
		files["files/"+p] = b
	}
	for p, b := range files {
		if err := writeAtomic(filepath.Join(tmp, filepath.FromSlash(p)), b); err != nil {
			return err
		}
	}
	_ = os.RemoveAll(final)
	return os.Rename(tmp, final)
}

// Retain removes every package not in keep.
func (s *Store) Retain(keep []ni.Digest) error {
	k := map[string]bool{}
	for _, d := range keep {
		k[d.Segment()] = true
	}
	entries, err := os.ReadDir(filepath.Join(s.dir, "packages"))
	if err != nil {
		return err
	}
	for _, e := range entries {
		if !k[e.Name()] {
			_ = os.RemoveAll(filepath.Join(s.dir, "packages", e.Name()))
		}
	}
	return nil
}

// agreements reads agreements.json.
func (s *Store) agreements() (map[string]domain.Entry, error) {
	out := map[string]domain.Entry{}
	b, err := os.ReadFile(filepath.Join(s.dir, "agreements.json"))
	if errors.Is(err, os.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	return out, json.Unmarshal(b, &out)
}

// PutAgreement stores an agreement entry.
func (s *Store) PutAgreement(e domain.Entry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	all, err := s.agreements()
	if err != nil {
		return err
	}
	all[e.AgreementID] = e
	b, _ := json.Marshal(all)
	return writeAtomic(filepath.Join(s.dir, "agreements.json"), b)
}

// Agreements adapts the store to AgreementRepo.
func (s *Store) Agreements() app.AgreementRepo { return agreementRepo{s} }

// agreementRepo exposes agreement methods under the AgreementRepo names.
type agreementRepo struct{ s *Store }

// Put stores an entry.
func (a agreementRepo) Put(e domain.Entry) error { return a.s.PutAgreement(e) }

// Get returns an entry.
func (a agreementRepo) Get(id string) (domain.Entry, bool, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	all, err := a.s.agreements()
	e, ok := all[id]
	return e, ok, err
}

// All returns every entry.
func (a agreementRepo) All() ([]domain.Entry, error) {
	a.s.mu.Lock()
	defer a.s.mu.Unlock()
	all, err := a.s.agreements()
	var out []domain.Entry
	for _, e := range all {
		out = append(out, e)
	}
	return out, err
}
