// Composition root of alexandria-client: checks CFG-1 and CFG-2, wires internal/client.Module, and serves traffic and admin listeners.
// Exits with status 2 and the failing code when the configuration is invalid (REQ-CLI-001, REQ-SYS-002, DP-5, DP-8).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ging/alexandria/internal/client"
	"github.com/ging/alexandria/internal/client/config"
	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/timex"
)

// main parses flags, validates configuration, and starts the alexandria-client process.
func main() {
	cfgPath := flag.String("config", "deploy/client.json", "path to the client configuration (Appendix A.3, JSON or YAML)")
	stateDir := flag.String("state-dir", "alexandria-client-state", "directory for continuity, packages and agreements")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	insecure := flag.Bool("did.allow_insecure_localhost", false, "allow http:// for a localhost did:web (PoC only, DP-5)")
	hubCA := flag.String("hub-ca", "", "PEM root certificate of a lab hub (PoC only, DP-14)")
	flag.Parse()
	cfg, cerr := config.Load(*cfgPath)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "alexandria-client: %s: %s\n", cerr.Code, cerr.Detail)
		os.Exit(2)
	}
	if err := run(cfg, *stateDir, *clockFile, *hubCA, *insecure); err != nil {
		if ce, ok := client.Coded(err); ok {
			fmt.Fprintf(os.Stderr, "alexandria-client: %s: %s\n", ce.Code, ce.Detail)
			os.Exit(2)
		}
		fmt.Fprintln(os.Stderr, "alexandria-client:", err)
		os.Exit(1)
	}
}

// run wires the client module and blocks until a termination signal.
func run(cfg *config.Config, stateDir, clockFile, hubCA string, insecure bool) error {
	var clk timex.Clock = clock.System{}
	if clockFile != "" {
		fc, err := clock.NewFile(clockFile)
		if err != nil {
			return err
		}
		clk = fc
	}
	mod, err := client.New(client.Deps{
		Config:                 cfg,
		StateDir:               stateDir,
		Clock:                  clk,
		AllowInsecureLocalhost: insecure,
		HubCA:                  hubCA,
	})
	if err != nil {
		return err
	}
	defer func() { _ = mod.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	handlers := mod.Handlers()
	errs := make(chan error, len(cfg.Listeners))
	var servers []*http.Server
	for name, l := range cfg.Listeners {
		ln, err := new(net.ListenConfig).Listen(ctx, "tcp", l.Bind)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		srv := &http.Server{Handler: handlers[name], ReadHeaderTimeout: 10 * time.Second}
		servers = append(servers, srv)
		go func(name string) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("%s: %w", name, err)
			}
		}(name)
		log.Printf("alexandria-client %s listening on %s", name, l.Bind)
	}
	if err := mod.Start(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
	case err := <-errs:
		return err
	}
	for _, s := range servers {
		_ = s.Shutdown(context.Background())
	}
	return nil
}
