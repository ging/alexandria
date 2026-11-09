// Tests of the content-addressed blob store.
// Bytes are only accepted under their own SHA-256 and are written read-only (REQ-SYS-005).
package blob_fs

import (
	"os"
	"testing"

	"github.com/ging/alexandria/vhub/internal/core/ni"
)

func TestPutGetByHash(t *testing.T) {
	s, _ := New(t.TempDir())
	b := []byte("blob")
	if err := s.Put(ni.HexSum(b), b); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ni.HexSum(b))
	if err != nil || string(got) != "blob" {
		t.Fatalf("got %q %v", got, err)
	}
	if fi, _ := os.Stat(s.path(ni.HexSum(b))); fi.Mode().Perm()&0o222 != 0 {
		t.Fatal("blob is writable")
	}
}

func TestPutRejectsWrongHash(t *testing.T) {
	s, _ := New(t.TempDir())
	if err := s.Put(ni.HexSum([]byte("a")), []byte("b")); err == nil {
		t.Fatal("wrong hash accepted")
	}
}
