// Command demo runs the end-to-end F6 scenario against the compiled binaries and prints the verdict table.
// Launches fake-idp, fake-connector, alexandria-server and alexandria-client with a shared file clock (Part 4 M9, DoD 6).
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ging/alexandria/conformance/f6"
)

// main parses flags and runs the F6 demo scenario.
func main() {
	binDir := flag.String("bin-dir", "bin", "directory containing alexandria-server, alexandria-client, fake-idp and fake-connector")
	flag.Parse()
	if err := run(*binDir); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

// run creates a temporary workspace, ensures the four binaries exist, launches them and replays F6.1–F6.4.
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
	fmt.Printf("%-20s %-34s %3s %-28s %s\n", "INSTANT", "STEP", "ST", "CODE / DETAILS", "VHUB-VERDICT")
	fmt.Println(strings.Repeat("-", 120))
	env, stop, err := f6.Launch(actualBinDir, work, io.Discard)
	if err != nil {
		return err
	}
	defer stop()
	r := &f6.Runner{E: env, Log: os.Stdout}
	stopClient := func() {}
	err = r.Run(func() error {
		s, err := env.StartClient(io.Discard)
		stopClient = s
		return err
	})
	defer stopClient()
	if err != nil {
		return err
	}
	want := f6.Expected()
	if len(r.Rows) != len(want) {
		return fmt.Errorf("got %d rows, want %d", len(r.Rows), len(want))
	}
	for i := range want {
		if r.Rows[i] != want[i] {
			return fmt.Errorf("row %d mismatch:\n got  %s\n want %s", i, r.Rows[i], want[i])
		}
	}
	fmt.Println(strings.Repeat("-", 120))
	fmt.Printf("F6 scenario completed: %d/%d rows verified.\n", len(r.Rows), len(want))
	return nil
}

// ensureBinaries checks if the four binaries exist in binDir and builds them into fallbackDir if missing.
func ensureBinaries(binDir, fallbackDir string) (string, error) {
	targets := map[string]string{
		"alexandria-server": "./cmd/alexandria-server",
		"alexandria-client": "./cmd/alexandria-client",
		"fake-idp":          "./deploy/fake-idp",
		"fake-connector":    "./deploy/fake-connector",
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
	if err := os.MkdirAll(fallbackDir, 0o750); err != nil {
		return "", err
	}
	for name, pkg := range targets {
		out, err := exec.CommandContext(context.Background(), "go", "build", "-o", filepath.Join(fallbackDir, name), pkg).CombinedOutput() // #nosec G204 -- fixed package list of this repository.
		if err != nil {
			return "", fmt.Errorf("build %s: %w\n%s", name, err, out)
		}
	}
	return fallbackDir, nil
}
