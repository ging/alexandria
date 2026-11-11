// Shared setup of the conformance battery: builds the four binaries once into a temporary directory.
// The e2e test runs them as separate processes, like an operator would (Part 4 M7, M8).
package conformance

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// binDir holds the binaries built by TestMain.
var binDir string

// TestMain builds alexandria-server, alexandria-client, fake-idp and fake-connector.
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vhub-bin-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	binDir = dir
	for name, pkg := range map[string]string{
		"alexandria-server": "./cmd/alexandria-server", "alexandria-client": "./cmd/alexandria-client",
		"fake-idp": "./deploy/fake-idp", "fake-connector": "./deploy/fake-connector",
	} {
		cmd := exec.Command("go", "build", "-o", filepath.Join(dir, name), pkg) // #nosec G204 -- fixed package list.
		cmd.Dir, cmd.Stdout, cmd.Stderr = "..", os.Stderr, os.Stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintln(os.Stderr, "build", name, err)
			os.Exit(1)
		}
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// bin returns the path of a built binary.
func bin(name string) string { return filepath.Join(binDir, name) }
