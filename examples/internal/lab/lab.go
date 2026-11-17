// Package lab runs the F6 scenario for the examples: hub, identity provider, connector and alexandria-client in the chosen deployment mode.
// It reuses the conformance launcher, so both topologies must print the same verdict table (see examples/README.md).
package lab

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/ging/alexandria/conformance/f6"
)

// Options selects the topology of one example run.
type Options struct {
	Title     string
	BinDir    string
	Validator bool
	Keep      bool
	Verbose   bool
	// Front starts the external proxy on env.Ingress once the client is up; nil when the client proxies itself.
	Front func(env f6.Env) (stop func(), err error)
}

// Run launches the lab, replays F6 through the topology, checks the table and optionally keeps the lab up.
func Run(o Options) error {
	work, err := os.MkdirTemp("", "alexandria-example-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()
	binDir, err := f6.EnsureBinaries(o.BinDir, filepath.Join(work, "bin"))
	if err != nil {
		return err
	}
	logs := io.Discard
	if o.Verbose {
		logs = os.Stderr
	}
	env, stop, err := f6.Launch(binDir, work, logs)
	if err != nil {
		return err
	}
	defer stop()
	env.Validator = o.Validator

	fmt.Println(o.Title)
	fmt.Printf("%-20s %-34s %3s %-28s %s\n", "INSTANT", "STEP", "ST", "CODE / DETAILS", "VHUB-VERDICT")
	fmt.Println(strings.Repeat("-", 120))
	var stops []func()
	defer func() {
		for i := len(stops) - 1; i >= 0; i-- {
			stops[i]()
		}
	}()
	r := &f6.Runner{E: env, Log: os.Stdout}
	err = r.Run(func() error {
		s, err := env.StartClient(logs)
		if err != nil {
			return err
		}
		stops = append(stops, s)
		if o.Front == nil {
			return nil
		}
		s, err = o.Front(env)
		if err != nil {
			return err
		}
		stops = append(stops, s)
		return nil
	})
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
	if o.Keep {
		keep(env)
	}
	return nil
}

// keep prints the endpoints of the running lab and blocks until interrupted.
func keep(env f6.Env) {
	fmt.Println()
	fmt.Println("The lab stays up (Ctrl-C to stop); its clock is frozen at the last instant of the table.")
	fmt.Printf("  DSP ingress   %s\n", env.Ingress)
	if env.Validator {
		fmt.Printf("  Validator     http://%s   (what the external proxy asks)\n", env.Check)
	}
	fmt.Printf("  Client admin  %s/status\n", env.Admin)
	fmt.Printf("  Connector     %s/_received\n", env.ConnURL)
	fmt.Printf("  Hub           %s   (CA: %s)\n", env.HubURL, env.CAFile)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
}
