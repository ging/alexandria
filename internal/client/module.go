// Package client wires the vhub-client bounded context and manages its background synchronization.
// Edge is constructed without HubAPI or DIDResolver so the hot path never contacts the hub (REQ-CLI-001, REQ-CLI-003, REQ-SYS-002, DP-5).
package client

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/client/adapters/did_web"
	"github.com/ging/alexandria/internal/client/adapters/evaluator_poc"
	"github.com/ging/alexandria/internal/client/adapters/events"
	"github.com/ging/alexandria/internal/client/adapters/http_hub"
	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/adapters/proxy_dsp"
	"github.com/ging/alexandria/internal/client/adapters/receipt_scitt_fake"
	"github.com/ging/alexandria/internal/client/adapters/status_http"
	"github.com/ging/alexandria/internal/client/adapters/store_fs"
	"github.com/ging/alexandria/internal/client/adapters/sync_timer"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/config"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
)

// Name identifies the vhub-client bounded context in logs and startup reports.
const Name = "vhub-client"

// Deps groups the configuration and runtime dependencies for the client module.
type Deps struct {
	Config                 *config.Config
	StateDir               string
	Clock                  timex.Clock
	AllowInsecureLocalhost bool
	DIDResolver            app.DIDResolver
}

// Module is the assembled vhub-client context with its proxy/admin handlers and sync loop.
type Module struct {
	cfg        *config.Config
	syncer     *app.Sync
	status     *app.Status
	edge       *app.Edge
	agreements *app.Agreements
	audit      *app.Audit
	cell       *app.Cell
	handlers   map[string]http.Handler
	bg         sync.WaitGroup
	cancel     context.CancelFunc
}

// New assembles the vhub-client module and restores persisted continuity state.
func New(deps Deps) (*Module, error) {
	cfg := deps.Config
	store, err := store_fs.Open(deps.StateDir)
	if err != nil {
		return nil, err
	}
	sink, err := events.Open(cfg.Events.Sink)
	if err != nil {
		return nil, err
	}
	resolver := deps.DIDResolver
	if resolver == nil {
		resolver = did_web.New(deps.AllowInsecureLocalhost)
	}
	reg := metrics.New()
	cell := &app.Cell{}
	trust := app.NewHubTrust(did.DID(cfg.Trust.HubDID), resolver, deps.Clock, cfg.DIDTTLValue, sink, cfg.Space)
	ev := &evaluator_poc.Evaluator{Events: sink, Clock: deps.Clock, Space: cfg.Space}
	agreements := &app.Agreements{Repo: store.Agreements(), Events: sink, Clock: deps.Clock, Space: cfg.Space}
	audit := &app.Audit{Window: cfg.Audit.Window, Evaluator: ev, Events: sink, Clock: deps.Clock, Space: cfg.Space}
	status := &app.Status{
		Cell:     cell,
		Clock:    deps.Clock,
		Trust:    trust,
		Packages: store,
		Events:   sink,
		Space:    cfg.Space,
		HubDID:   cfg.Trust.HubDID,
		Gates:    cfg.Gates(),
	}
	syncer := &app.Sync{
		Hub:        http_hub.New(hubBase(cfg.Trust.Hub, deps.AllowInsecureLocalhost), cfg.Token),
		Trust:      trust,
		Receipts:   receipt_scitt_fake.Verifier{},
		Continuity: store,
		Packages:   store,
		Agreements: agreements,
		Audit:      audit,
		Status:     status,
		Cell:       cell,
		Clock:      deps.Clock,
		Events:     sink,
		Space:      cfg.Space,
	}
	if err := syncer.Restore(); err != nil {
		return nil, err
	}
	up, err := proxy_dsp.NewUpstream(cfg.Upstreams.DSP, cfg.Upstreams.Management)
	if err != nil {
		return nil, err
	}
	edge := &app.Edge{
		Cell:             cell,
		Clock:            deps.Clock,
		Evaluator:        ev,
		Upstream:         up,
		Agreements:       agreements,
		Audit:            audit,
		Events:           sink,
		Metrics:          reg,
		Routes:           cfg.Routes,
		DefaultAction:    cfg.DefaultAction,
		UnknownAgreement: cfg.UnknownAgreement,
		Space:            cfg.Space,
	}
	handlers := map[string]http.Handler{
		"admin": status_http.Admin{Status: status, Agreements: agreements, Audit: audit, Cell: cell, Metrics: reg}.Handler(),
	}
	for _, l := range []domain.Listener{domain.DSPIngress, domain.MgmtIngress, domain.DSPEgress} {
		handlers[string(l)] = proxy_dsp.Handler{Edge: edge, Listener: l, Latency: reg}
	}
	return &Module{
		cfg:        cfg,
		syncer:     syncer,
		status:     status,
		edge:       edge,
		agreements: agreements,
		audit:      audit,
		cell:       cell,
		handlers:   handlers,
	}, nil
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

// Name returns the module identifier.
func (m *Module) Name() string { return Name }

// Handlers returns the HTTP handler map keyed by listener name.
func (m *Module) Handlers() map[string]http.Handler { return m.handlers }

// Sync exposes the synchronization use case.
func (m *Module) Sync() *app.Sync { return m.syncer }

// Status exposes the status use case.
func (m *Module) Status() *app.Status { return m.status }

// Edge exposes the hot-path validation use case.
func (m *Module) Edge() *app.Edge { return m.edge }

// Describe returns the configured dataspace IRI for startup reporting.
func (m *Module) Describe() (string, bool) { return m.cfg.Space, m.cfg.Space != "" }

// Start launches the background synchronization and state watcher loop.
func (m *Module) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.bg.Add(1)
	go func() {
		defer m.bg.Done()
		sync_timer.Run(ctx, m.syncer, m.status, time.Second)
	}()
	return nil
}

// Close stops the background synchronization loop and waits for it to finish.
func (m *Module) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.bg.Wait()
	return nil
}
