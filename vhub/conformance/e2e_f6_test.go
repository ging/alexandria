// End-to-end reproduction of the F6 table with two real binaries, a fake connector and a shared file clock.
// Every row of F6 must match exactly, including the VHub-Verdict header of m2 and the A1 revocation (Part 4 M8, REQ-SYS-001, REQ-SYS-007).
package conformance

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/ging/alexandria/vhub/conformance/f6"
)

// testLog adapts testing.T to io.Writer.
type testLog struct{ t *testing.T }

// Write logs one line.
func (l testLog) Write(p []byte) (int, error) { l.t.Log(string(p)); return len(p), nil }

func TestE2E_F6(t *testing.T) {
	work := t.TempDir()
	env, stop, err := f6.Launch(binDir, work, testLog{t})
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	got, err := f6.Run(env)
	if err != nil {
		b, _ := os.ReadFile(filepath.Join(work, "vhub-client.log"))
		t.Fatalf("%v\nclient log:\n%s", err, b)
	}
	want := f6.Expected()
	if len(got) != len(want) {
		t.Fatalf("got %d rows, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("row %d:\n got  %s\n want %s", i, got[i], want[i])
		}
	}
}
