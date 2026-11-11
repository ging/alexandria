// Tests the package store: a package round-trips with nested paths, and List and Delete see it (REQ-CLI-017, REQ-CLI-033).
// Fixtures: a temporary directory and a two-file package.
package package_fs

import (
	"bytes"
	"testing"

	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
)

// files is a package with a nested path.
var files = pkgv.Files{Manifest: []byte(`{"m":1}`), Content: map[string][]byte{"schemas/a.json": []byte(`{}`), "profile/slots.json": []byte(`[]`)}}

func TestRoundTripListDelete(t *testing.T) {
	s, _ := Open(t.TempDir())
	d := ni.Sum(files.Manifest)
	if err := s.Put(d, files); err != nil {
		t.Fatal(err)
	}
	got, ok, err := s.Get(d)
	if !ok || err != nil || !bytes.Equal(got.Content["schemas/a.json"], []byte(`{}`)) || len(got.Content) != 2 {
		t.Fatalf("got %+v %v %v", got, ok, err)
	}
	if l, _ := s.List(); len(l) != 1 || l[0] != d {
		t.Fatalf("list %v", l)
	}
	_ = s.Delete(d)
	if _, ok, _ := s.Get(d); ok {
		t.Fatal("deleted package still present")
	}
}
