// Command demo runs the end-to-end F6 scenario against the compiled binaries and prints the verdict table.
// Launches fake-connector, vhub-server, and vhub-client with a shared file clock (Part 4 M9, DoD item 6).
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ging/alexandria/conformance/f6"
)

// main parses flags and runs the F6 demo scenario.
func main() {
	binDir := flag.String("bin-dir", "bin", "directory containing vhub-server, vhub-client, and fake-connector")
	flag.Parse()
	if err := run(*binDir); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

// run creates a temporary workspace, builds binaries if absent, launches the three processes, and executes F6.
func run(binDir string) error {
	work, err := os.MkdirTemp("", "vhub-demo-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	actualBinDir, err := ensureBinaries(binDir, filepath.Join(work, "bin"))
	if err != nil {
		return err
	}
	fmt.Printf("%-22s %-28s %-10s %-15s %-6s %s\n", "INSTANT", "MESSAGE", "CLASS", "ACTION", "HTTP", "VERDICT / CODE")
	fmt.Println(strings.Repeat("-", 120))
	env, stop, err := f6.Launch(actualBinDir, work, os.Stdout)
	if err != nil {
		return err
	}
	defer stop()
	rows, err := f6.Run(env)
	if err != nil {
		return err
	}
	want := f6.Expected()
	if len(rows) != len(want) {
		return fmt.Errorf("got %d rows, want %d", len(rows), len(want))
	}
	for i := range want {
		if rows[i] != want[i] {
			return fmt.Errorf("row %d mismatch:\n got  %s\n want %s", i, rows[i], want[i])
		}
	}
	fmt.Println(strings.Repeat("-", 120))
	fmt.Printf("F6 scenario completed: %d/%d rows verified.\n", len(rows), len(want))
	return nil
}

// ensureBinaries checks if the three binaries exist in binDir and builds them into fallbackDir if missing.
func ensureBinaries(binDir, fallbackDir string) (string, error) {
	targets := map[string]string{
		"vhub-server":    "./cmd/vhub-server",
		"vhub-client":    "./cmd/vhub-client",
		"fake-connector": "./deploy/fake-connector",
	}
	allExist := true
	for name := range targets {
		if _, err := os.Stat(filepath.Join(binDir, name)); err != nil {
			allExist = false
			break
		}
	}
	if allExist {
		return filepath.Abs(binDir)
	}
	if err := os.MkdirAll(fallbackDir, 0o755); err != nil {
		return "", err
	}
	for name, pkg := range targets {
		out, err := exec.Command("go", "build", "-o", filepath.Join(fallbackDir, name), pkg).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("build %s: %v\n%s", name, err, out)
		}
	}
	return fallbackDir, nil
}
