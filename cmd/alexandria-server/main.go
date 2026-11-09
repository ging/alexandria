// Composition root of alexandria-server: loads server.json, wires internal/server.Module, and serves HTTP.
// Mounts /v1, /pub and /.well-known/did.json and starts the listing scheduler (REQ-HUB-008, DP-2, DP-3, DP-4, DP-8, DP-9, DP-12).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server"
	"github.com/ging/alexandria/internal/server/config"
)

// main parses flags and starts the alexandria-server process.
func main() {
	cfgPath := flag.String("config", "config/server.json", "path to server.json")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	flag.Parse()
	if err := run(*cfgPath, *clockFile); err != nil {
		fmt.Fprintln(os.Stderr, "alexandria-server:", err)
		os.Exit(1)
	}
}

// run builds the server module and blocks until a termination signal.
func run(cfgPath, clockFile string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if clockFile != "" {
		cfg.ClockFile = clockFile
	}
	var clk timex.Clock = clock.System{}
	if cfg.ClockFile != "" {
		if clk, err = clock.NewFile(cfg.ClockFile); err != nil {
			return err
		}
	}
	mod, err := server.New(server.Deps{Config: cfg, Clock: clk})
	if err != nil {
		return err
	}
	defer func() { _ = mod.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := mod.Start(ctx); err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.Listen, Handler: mod.Handler(), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Printf("alexandria-server listening on %s as %s (PoC: not for production)", cfg.Listen, cfg.DID)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
