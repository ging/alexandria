// Composition root of alexandria-client: checks CFG-1 and CFG-2, wires internal/client.Module, and serves traffic and admin listeners.
// Exits with status 2 and the failing code when the configuration is invalid (REQ-CLI-001, REQ-SYS-002, DP-5, DP-8).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ging/alexandria/internal/client"
	"github.com/ging/alexandria/internal/client/config"
	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/telemetry"
)

// main parses flags, validates configuration, and starts the alexandria-client process.
func main() {
	cfgPath := flag.String("config", "config/client.yaml", "path to the client configuration (YAML or JSON)")
	stateDir := flag.String("state-dir", "alexandria-client-state", "directory for continuity, packages and agreements")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	insecure := flag.Bool("did.allow_insecure_localhost", false, "allow http:// for a localhost did:web (PoC only, DP-5)")
	hubCA := flag.String("hub-ca", "", "PEM root certificate of a lab hub (PoC only, DP-14)")
	logLevel := flag.String("log-level", "", "override observability.log_level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "", "override observability.log_format (auto, json, text)")
	flag.Parse()
	cfg, cerr := config.Load(*cfgPath)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "alexandria-client: %s: %s\n", cerr.Code, cerr.Detail)
		os.Exit(2)
	}
	if *logLevel != "" {
		cfg.Log.Level = *logLevel
	}
	if *logFormat != "" {
		cfg.Log.Format = *logFormat
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
	if hubCA == "" {
		hubCA = cfg.HubCA
	}
	svcName := cfg.Log.ServiceName
	if svcName == "" {
		svcName = client.Name
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rt, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:  svcName,
		Environment:  cfg.Log.Environment,
		Level:        cfg.Log.Level,
		Format:       cfg.Log.Format,
		OTLPEndpoint: cfg.Log.OTLPEndpoint,
		OTLPInsecure: cfg.Log.OTLPInsecure,
		Clock:        clk,
		SetGlobal:    true,
	})
	if err != nil {
		return err
	}
	defer func() {
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = rt.Shutdown(sctx)
	}()

	mod, err := client.New(client.Deps{
		Config:                 cfg,
		StateDir:               stateDir,
		Clock:                  clk,
		AllowInsecureLocalhost: insecure,
		HubCA:                  hubCA,
		Logger:                 rt.Logger,
		Tracer:                 rt.Tracer,
		Propagator:             rt.Propagator,
	})
	if err != nil {
		rt.Logger.ErrorContext(ctx, "client module initialization failed", logx.Err(err))
		return err
	}
	defer func() { _ = mod.Close() }()
	handlers := mod.Handlers()
	errs := make(chan error, len(cfg.Listeners))
	var servers []*http.Server
	for name, l := range cfg.Listeners {
		ln, err := new(net.ListenConfig).Listen(ctx, "tcp", l.Bind)
		if err != nil {
			rt.Logger.ErrorContext(ctx, "listener bind failed", slog.String("listener", name), slog.String("bind", l.Bind), logx.Err(err))
			return fmt.Errorf("%s: %w", name, err)
		}
		srv := &http.Server{
			Handler:           handlers[name],
			ErrorLog:          rt.ErrorLog(),
			ReadHeaderTimeout: 10 * time.Second,
		}
		servers = append(servers, srv)
		go func(name string) {
			if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
				errs <- fmt.Errorf("%s: %w", name, err)
			}
		}(name)
		rt.Logger.InfoContext(ctx, "listener started", slog.String("listener", name), slog.String("bind", l.Bind), slog.String("namespace", cfg.NamespaceIRI()))
	}
	if err := mod.Start(ctx); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		rt.Logger.Info("shutting down client listeners")
	case err := <-errs:
		rt.Logger.ErrorContext(ctx, "listener terminated with error", logx.Err(err))
		return err
	}
	sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	for _, s := range servers {
		_ = s.Shutdown(sctx)
	}
	rt.Logger.Info("client stopped")
	return nil
}
