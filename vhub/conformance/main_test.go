// Test entry of the conformance battery: builds the real binaries once into a temporary directory.
// The e2e and binary-level negative cases run these binaries (Part 4 M7, M8).
package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binDir holds vhub-server, vhub-client and fake-connector built for this run.
var binDir string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vhub-bin-")
	if err != nil {
		panic(err)
	}
	binDir = dir
	cmd := exec.Command("go", "build", "-o", dir+string(filepath.Separator), "./cmd/vhub-server", "./cmd/vhub-client", "./deploy/fake-connector")
	cmd.Dir = ".."
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "build: %v\n%s", err, out)
		os.Exit(1)
	}
	code := m.Run()
	stopShared()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}
