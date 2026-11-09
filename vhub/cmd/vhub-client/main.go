// Composition root of vhub-client: checks CFG-1..CFG-5, builds the adapters and serves the traffic and admin listeners.
// Exits with status 2 and the failing code when the configuration is invalid (REQ-CLI-001, REQ-SYS-002, DP-5, DP-9).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ging/alexandria/vhub/internal/client/adapters/did_web"
	"github.com/ging/alexandria/vhub/internal/client/adapters/evaluator_poc"
	"github.com/ging/alexandria/vhub/internal/client/adapters/events"
	"github.com/ging/alexandria/vhub/internal/client/adapters/http_hub"
	"github.com/ging/alexandria/vhub/internal/client/adapters/metrics"
	"github.com/ging/alexandria/vhub/internal/client/adapters/proxy_dsp"
	"github.com/ging/alexandria/vhub/internal/client/adapters/receipt_scitt_fake"
	"github.com/ging/alexandria/vhub/internal/client/adapters/status_http"
	"github.com/ging/alexandria/vhub/internal/client/adapters/store_fs"
	"github.com/ging/alexandria/vhub/internal/client/adapters/sync_timer"
	"github.com/ging/alexandria/vhub/internal/client/app"
	"github.com/ging/alexandria/vhub/internal/client/config"
	"github.com/ging/alexandria/vhub/internal/client/domain"
	"github.com/ging/alexandria/vhub/internal/clock"
	"github.com/ging/alexandria/vhub/internal/core/did"
	"github.com/ging/alexandria/vhub/internal/core/timex"
)

func main() {
	cfgPath := flag.String("config", "config.json", "path to the client configuration (Appendix A.3)")
	stateDir := flag.String("state-dir", "vhub-client-state", "directory for continuity, packages and agreements")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	insecure := flag.Bool("did.allow_insecure_localhost", false, "allow http:// for a localhost hub and its did:web (PoC only, DP-5)")
	flag.Parse()
	cfg, cerr := config.Load(*cfgPath)
	if cerr != nil {
		fmt.Fprintf(os.Stderr, "vhub-client: %s: %s\n", cerr.Code, cerr.Detail)
		os.Exit(2)
	}
	if err := run(cfg, *stateDir, *clockFile, *insecure); err != nil {
		fmt.Fprintln(os.Stderr, "vhub-client:", err)
		os.Exit(1)
	}
}

// hubBase returns the hub URL, downgraded to http for localhost when the PoC flag is set.
func hubBase(hub string, insecure bool) string {
	u, err := url.Parse(hub)
	if err != nil || !insecure || !did.IsLocalhost(u.Host) {
		return hub
	}
	u.Scheme = "http"
	return u.String()
}

// run wires the client and blocks until a termination signal.
func run(cfg *config.Config, stateDir, clockFile string, insecure bool) error {
	var clk timex.Clock = clock.System{}
	if clockFile != "" {
		fc, err := clock.NewFile(clockFile)
		if err != nil {
			return err
		}
		clk = fc
	}
	store, err := store_fs.Open(stateDir)
	if err != nil {
		return err
	}
	sink, err := events.Open(cfg.Events.Sink)
	if err != nil {
		return err
	}
	reg := metrics.New()
	cell := &app.Cell{}
	trust := app.NewHubTrust(did.DID(cfg.Trust.HubDID), did_web.New(insecure), clk, cfg.DIDTTLValue, sink, cfg.Space)
	ev := &evaluator_poc.Evaluator{Events: sink, Clock: clk, Space: cfg.Space}
	agreements := &app.Agreements{Repo: store.Agreements(), Events: sink, Clock: clk, Space: cfg.Space}
	audit := &app.Audit{Window: cfg.Audit.Window, Evaluator: ev, Events: sink, Clock: clk, Space: cfg.Space}
	status := &app.Status{Cell: cell, Clock: clk, Trust: trust, Packages: store, Events: sink, Space: cfg.Space, HubDID: cfg.Trust.HubDID, Gates: cfg.Gates()}
	syncer := &app.Sync{Hub: http_hub.New(hubBase(cfg.Trust.Hub, insecure), cfg.Token), Trust: trust, Receipts: receipt_scitt_fake.Verifier{},
		Continuity: store, Packages: store, Agreements: agreements, Audit: audit, Status: status, Cell: cell, Clock: clk, Events: sink, Space: cfg.Space}
	if err := syncer.Restore(); err != nil {
		return err
	}
	up, err := proxy_dsp.NewUpstream(cfg.Upstreams.DSP, cfg.Upstreams.Management)
	if err != nil {
		return err
	}
	edge := &app.Edge{Cell: cell, Clock: clk, Evaluator: ev, Upstream: up, Agreements: agreements, Audit: audit, Events: sink, Metrics: reg,
		Routes: cfg.Routes, DefaultAction: cfg.DefaultAction, UnknownAgreement: cfg.UnknownAgreement, Space: cfg.Space}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	handlers := map[string]http.Handler{"admin": status_http.Admin{Status: status, Agreements: agreements, Audit: audit, Cell: cell, Metrics: reg}.Handler()}
	for _, l := range []domain.Listener{domain.DSPIngress, domain.MgmtIngress, domain.DSPEgress} {
		handlers[string(l)] = proxy_dsp.Handler{Edge: edge, Listener: l, Latency: reg}
	}
	errs := make(chan error, len(cfg.Listeners))
	var servers []*http.Server
	for name, l := range cfg.Listeners {
		ln, err := net.Listen("tcp", l.Bind)
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
		log.Printf("vhub-client %s listening on %s", name, l.Bind)
	}
	go sync_timer.Run(ctx, syncer, status, time.Second)
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
