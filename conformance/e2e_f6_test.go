// End-to-end reproduction of F6 with the real binaries, fake-idp, fake-connector and a shared file clock.
// Every row of F6.1–F6.4 must match exactly, including the VHub-Verdict headers and the A1 revocation (Part 4 M8, REQ-SYS-001, REQ-SYS-007).
package conformance

import (
	"bytes"
	"testing"

	"github.com/ging/alexandria/conformance/f6"
)

func TestE2E_F6(t *testing.T) {
	var log bytes.Buffer
	env, stop, err := f6.Launch(binDir, t.TempDir(), &log)
	if err != nil {
		t.Fatalf("launch: %v\n%s", err, log.String())
	}
	defer stop()
	run := &f6.Runner{E: env}
	stopClient := func() {}
	err = run.Run(func() error {
		s, err := env.StartClient(&log)
		stopClient = s
		return err
	})
	defer stopClient()
	if err != nil {
		t.Fatalf("scenario: %v\n%s", err, log.String())
	}
	want := f6.Expected()
	if len(run.Rows) != len(want) {
		t.Fatalf("got %d rows, want %d\n%v", len(run.Rows), len(want), run.Rows)
	}
	for i := range want {
		if run.Rows[i] != want[i] {
			t.Errorf("row %d:\n got  %s\n want %s", i, run.Rows[i], want[i])
		}
	}
}
