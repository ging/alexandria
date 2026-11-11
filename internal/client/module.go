// Package client wires the alexandria-client bounded context: stores, hub client, trust, sync, rules, edge, GC and the admin API.
// Edge and Explainer are built without the hub client or DID resolver (DP-2, DP-5, DP-7, REQ-SYS-002, REQ-CLI-001, REQ-CLI-019).
package client

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/client/adapters/did_resolver"
	"github.com/ging/alexandria/internal/client/adapters/evaluator_poc"
	"github.com/ging/alexandria/internal/client/adapters/events"
	"github.com/ging/alexandria/internal/client/adapters/http_hub"
	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/adapters/package_fs"
	"github.com/ging/alexandria/internal/client/adapters/proxy_dsp"
	"github.com/ging/alexandria/internal/client/adapters/status_http"
	"github.com/ging/alexandria/internal/client/adapters/store_sqlite"
	"github.com/ging/alexandria/internal/client/adapters/sync_timer"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/config"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
)

// Name identifies the alexandria-client bounded context in logs and startup reports.
const Name = "alexandria-client"

// Deps groups the configuration and runtime options of the client module.
type Deps struct {
	Config                 *config.Config
	StateDir               string
	Clock                  timex.Clock
	AllowInsecureLocalhost bool
	HubCA                  string
	Resolver               app.DIDResolver
}

// Module is the assembled alexandria-client context.
type Module struct {
	cfg      *config.Config
	syncer   *app.Sync
	gc       *app.GC
	status   *app.Status
	edge     *app.Edge
	store    *store_sqlite.Store
	handlers map[string]http.Handler
	bg       sync.WaitGroup
	cancel   context.CancelFunc
}

// New assembles the module; a failing CFG-2 is returned as a coded error.
func New(deps Deps) (*Module, error) {
	cfg := deps.Config
	if err := os.MkdirAll(deps.StateDir, 0o750); err != nil {
		return nil, err
	}
	store, err := store_sqlite.Open(filepath.Join(deps.StateDir, "client.db"))
	if err != nil {
		return nil, err
	}
	pkgs, err := package_fs.Open(filepath.Join(deps.StateDir, "packages"))
	if err != nil {
		return nil, err
	}
	sink, err := events.Open(cfg.Events.Sink)
	if err != nil {
		return nil, err
	}
	ns, hubDID := cfg.NamespaceIRI(), did.DID(cfg.Trust.HubDID)
	resolver := deps.Resolver
	if resolver == nil {
		resolver = did_resolver.New(deps.AllowInsecureLocalhost)
	}
	hub, err := http_hub.New(cfg.Trust.Hub, cfg.Trust.Namespace, deps.HubCA)
	if err != nil {
		return nil, err
	}
	reg := metrics.New()
	cell, rulesCell := &app.Cell{}, &app.RulesCell{}
	trust := app.NewHubTrust(hubDID, resolver, deps.Clock, cfg.DIDTTL, sink, ns)
	agreements := &app.Agreements{Repo: store_sqlite.Agreements{S: store}, Events: sink, Clock: deps.Clock, Namespace: ns}
	ev := &evaluator_poc.Evaluator{Events: sink, Clock: deps.Clock, Namespace: ns}
	audit := &app.Audit{Window: cfg.Audit.Window, Evaluator: ev, Events: sink, Clock: deps.Clock, Namespace: ns}
	status := &app.Status{Cell: cell, Rules: rulesCell, Trust: trust, Packages: pkgs, Events: sink, Clock: deps.Clock, Namespace: ns, HubDID: hubDID}
	syncer := &app.Sync{
		Hub: hub, Trust: trust, Continuity: store, Packages: pkgs, Agreements: agreements, Audit: audit, Status: status,
		Cell: cell, Clock: deps.Clock, Events: sink, Namespace: ns, HubDID: hubDID, Context: cfg.Trust.Context,
	}
	rules := &app.Rules{Store: store_sqlite.Rules{S: store}, Cell: rulesCell, Events: sink, Clock: deps.Clock, Namespace: ns, HasManagement: cfg.Upstreams.Management != ""}
	if cerr := rules.Init(cfg.Rules.Seed); cerr != nil {
		return nil, cerr
	}
	if err := syncer.Restore(); err != nil {
		return nil, err
	}
	up, err := proxy_dsp.NewUpstream(cfg.Upstreams.DSP, cfg.Upstreams.Management)
	if err != nil {
		return nil, err
	}
	explainer := app.Explainer{
		Snap: cell, Rules: rulesCell, Evaluator: ev, Agreements: store_sqlite.Agreements{S: store}, Clock: deps.Clock,
		Settings: app.Settings{Namespace: ns, DefaultAction: cfg.DefaultAction, UnknownAgreement: cfg.UnknownAgreement},
	}
	edge := &app.Edge{Explainer: explainer, Upstream: up, Capture: agreements, Audit: audit, Events: sink, Metrics: reg}
	gc := &app.GC{Packages: pkgs, Agreements: agreements, Cell: cell, Events: sink, Clock: deps.Clock, Namespace: ns, Interval: cfg.GCInterval}
	handlers := map[string]http.Handler{
		"admin": status_http.Admin{Status: status, Rules: rules, Explainer: &explainer, Agreements: agreements, Audit: audit, Cell: cell, Metrics: reg}.Handler(),
	}
	for _, l := range []domain.Listener{domain.DSPIngress, domain.MgmtIngress, domain.DSPEgress} {
		handlers[string(l)] = proxy_dsp.Handler{Edge: edge, Listener: l, Latency: reg}
	}
	return &Module{cfg: cfg, syncer: syncer, gc: gc, status: status, edge: edge, store: store, handlers: handlers}, nil
}

// Coded returns the coded error of a startup failure, if any.
func Coded(err error) (*codes.Error, bool) { return codes.As(err) }

// Name returns the module identifier.
func (m *Module) Name() string { return Name }

// Handlers returns the HTTP handler map keyed by listener name.
func (m *Module) Handlers() map[string]http.Handler { return m.handlers }

// Sync exposes the synchronization use case.
func (m *Module) Sync() *app.Sync { return m.syncer }

// Status exposes the status use case.
func (m *Module) Status() *app.Status { return m.status }

// Edge exposes the hot path.
func (m *Module) Edge() *app.Edge { return m.edge }

// Describe returns the namespace IRI for startup reporting.
func (m *Module) Describe() (string, bool) { return m.cfg.NamespaceIRI(), true }

// Start launches the sync, GC and state-watch loop.
func (m *Module) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.bg.Add(1)
	go func() {
		defer m.bg.Done()
		sync_timer.Run(ctx, m.syncer, m.gc, m.status, time.Second)
	}()
	return nil
}

// Close stops the background loop and closes the store.
func (m *Module) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.bg.Wait()
	return m.store.Close()
}
