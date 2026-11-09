// Shared run of the F6 scenario on the real binaries, reused by the e2e test and the document checks.
// The processes stay up until TestMain ends so later tests can inspect their outputs (Part 4 M7, M8).
package conformance

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ging/alexandria/conformance/f6"
)

// shared holds the single F6 run of this test binary.
var shared struct {
	once sync.Once
	env  f6.Env
	work string
	rows []f6.Row
	err  error
	stop func()
}

// f6Run launches the binaries and runs F6 once, returning the environment and observed rows.
func f6Run(t *testing.T) (f6.Env, []f6.Row, string) {
	t.Helper()
	shared.once.Do(func() {
		shared.work, _ = os.MkdirTemp("", "vhub-f6-")
		shared.env, shared.stop, shared.err = f6.Launch(binDir, shared.work, nil)
		if shared.err == nil {
			shared.rows, shared.err = f6.Run(shared.env)
		}
	})
	if shared.err != nil {
		b, _ := os.ReadFile(filepath.Join(shared.work, "alexandria-client.log"))
		t.Fatalf("%v\nclient log:\n%s", shared.err, b)
	}
	return shared.env, shared.rows, shared.work
}

// stopShared stops the shared processes and removes their state.
func stopShared() {
	if shared.stop != nil {
		shared.stop()
	}
	if shared.work != "" {
		_ = os.RemoveAll(shared.work)
	}
}
