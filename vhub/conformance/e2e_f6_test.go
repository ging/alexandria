// End-to-end reproduction of the F6 table with two real binaries, a fake connector and a shared file clock.
// Every row of F6 must match exactly, including the VHub-Verdict header of m2 and the A1 revocation (Part 4 M8, REQ-SYS-001, REQ-SYS-007).
package conformance

import (
	"testing"

	"github.com/ging/alexandria/vhub/conformance/f6"
)

func TestE2E_F6(t *testing.T) {
	_, got, _ := f6Run(t)
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
