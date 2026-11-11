// Package server wires the alexandria-server bounded context and manages its lifecycle.
// It connects SQLite, blobs, the file signer, the DID resolver, JWT/JWKS authentication and the /v1 and /pub handlers (DP-2, DP-4, DP-5, DP-11, REQ-HUB-008).
package server

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/dsl"
	"github.com/ging/alexandria/internal/server/adapters/blob_fs"
	"github.com/ging/alexandria/internal/server/adapters/compiler_dsl"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/adapters/did_resolver"
	"github.com/ging/alexandria/internal/server/adapters/fetch_http"
	"github.com/ging/alexandria/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/internal/server/adapters/http_v1"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/adapters/jwt_jwks"
	"github.com/ging/alexandria/internal/server/adapters/scheduler"
	"github.com/ging/alexandria/internal/server/adapters/signer_file"
	"github.com/ging/alexandria/internal/server/adapters/sqlite"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/config"
	"github.com/ging/alexandria/internal/telemetry"
)

// Name identifies the alexandria-server bounded context in logs and startup reports.
const Name = "alexandria-server"

// Deps groups the configuration and optional port overrides for the server module.
type Deps struct {
	Config     config.Server
	Clock      timex.Clock
	Signer     app.KeySigner
	Tokens     http_v1.TokenVerifier
	Resolver   app.DIDResolver
	Logger     *slog.Logger
	Tracer     trace.Tracer
	Propagator propagation.TextMapPropagator
}

// Module is the assembled alexandria-server context with its HTTP surface and background scheduler.
type Module struct {
	hub     *app.Hub
	handler http.Handler
	store   *sqlite.Store
	clock   timex.Clock
	logger  *slog.Logger
	did     did.DID
	listen  string
	keyTick timex.Duration
	bg      sync.WaitGroup
	cancel  context.CancelFunc
}

// New assembles the alexandria-server module from its configuration and dependencies.
func New(deps Deps) (*Module, error) {
	cfg := deps.Config
	for _, p := range []string{cfg.Storage.Database, cfg.Signing.Key} {
		if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
			return nil, err
		}
	}
	signer := deps.Signer
	hubDID := did.DID(cfg.DID)
	if signer == nil {
		fs, err := signer_file.Load(cfg.Signing.Key, cfg.Signing.VerificationMethod)
		if err != nil {
			return nil, err
		}
		if hubDID == "" {
			hubDID = fs.KeyDID()
		}
		signer = fs
	}
	if hubDID == "" || did.OfURL(signer.VerificationMethod()) != hubDID {
		return nil, errors.New("server.json: signing.verification_method must be a method of did")
	}
	store, err := sqlite.Open(cfg.Storage.Database)
	if err != nil {
		return nil, err
	}
	blobs, err := blob_fs.New(cfg.Storage.Blobs)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	resolver := deps.Resolver
	if resolver == nil {
		resolver = did_resolver.Resolver{AllowInsecureLocalhost: cfg.DIDResolver.AllowInsecureLocalhost}
	}
	logger := logx.Default(deps.Logger)
	hub := app.New(app.Deps{
		Store: store, Blobs: blobs, Signer: signer, Resolver: resolver, Compiler: profileCompiler(cfg), Clock: deps.Clock,
		Logger:  logx.Component(logger, "hub"),
		Fetcher: fetch_http.Fetcher{Timeout: time.Duration(cfg.Timeout) * time.Second, Allow: cfg.Vocab.ProxyAllow, AllowInsecureLocalhost: cfg.Vocab.AllowInsecureLocalhost},
		Base:    cfg.Base, DID: hubDID, Context: cfg.Context(), Defaults: cfg.Defaults, VocabMaxSize: cfg.Vocab.MaxSize,
		AllowInsecureLocalhost: cfg.Vocab.AllowInsecureLocalhost,
	})
	tokens := deps.Tokens
	if tokens == nil {
		tokens = verifier(cfg, deps.Clock, logx.Component(logger, "auth.jwks"))
	}
	mux := http.NewServeMux()
	http_v1.Handler{Hub: hub, Tokens: tokens, Context: cfg.Context(), Base: cfg.Base, Logger: logx.Component(logger, "http.v1")}.Register(mux)
	http_pub.Handler{Pub: hub.Publication, Problems: httpx.Problems{Context: cfg.Context(), Logger: logx.Component(logger, "http.pub")}}.Register(mux)
	hub.Keys.CheckSigningKey()
	wrapped := telemetry.HTTPMiddleware(telemetry.HTTPOptions{
		Logger:     logger,
		Tracer:     deps.Tracer,
		Propagator: deps.Propagator,
		Component:  "http",
		Listener:   "hub",
	}, httpx.WithDate(deps.Clock, mux))
	return &Module{
		hub: hub, handler: wrapped, store: store, clock: deps.Clock, logger: logger, did: hubDID,
		listen: cfg.Listen, keyTick: cfg.Defaults.Freshness.TTL / 2,
	}, nil
}

// verifier builds the jwt_jwks token verifier from server.json.
func verifier(cfg config.Server, c timex.Clock, logger *slog.Logger) *jwt_jwks.Verifier {
	v := &jwt_jwks.Verifier{
		Audience: cfg.Auth.Audience, Algorithms: cfg.Auth.Algorithms, AdminClaim: cfg.Auth.AdminClaim.Claim, AdminValue: cfg.Auth.AdminClaim.Value,
		Skew: cfg.Skew, CacheTTL: cfg.JWKSTTL, Clock: c, Logger: logger,
	}
	for _, i := range cfg.Auth.Issuers {
		v.Issuers = append(v.Issuers, jwt_jwks.Issuer{Iss: i.Iss, DiscoveryURL: i.DiscoveryURL})
	}
	return v
}

// profileCompiler selects the draft compiler: the PoC JSON drafts (poc, the default) or the YAML language (dsl).
func profileCompiler(cfg config.Server) app.ProfileCompiler {
	if cfg.Compiler != "dsl" {
		return compiler_poc.Compiler{Defaults: cfg.Defaults}
	}
	c := compiler_dsl.Compiler{}
	if cfg.VendorDir != "" {
		c.Vendor = dsl.FSVendor{FS: os.DirFS(cfg.VendorDir)}
	}
	return c
}

// Name returns the module identifier.
func (m *Module) Name() string { return Name }

// Hub exposes the assembled server use cases.
func (m *Module) Hub() *app.Hub { return m.hub }

// Handler returns the HTTP handler mounting /v1 and /pub.
func (m *Module) Handler() http.Handler { return m.handler }

// DID returns the hub DID.
func (m *Module) DID() did.DID { return m.did }

// Describe returns the hub DID for startup reporting.
func (m *Module) Describe() (string, bool) { return string(m.did), m.did != "" }

// Start launches the listing and signing-key scheduler in the background.
func (m *Module) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.bg.Add(1)
	go func() {
		defer m.bg.Done()
		scheduler.Run(ctx, m.hub, m.clock, time.Second, m.keyTick, m.logger)
	}()
	return nil
}

// Close stops the background scheduler and closes the underlying SQLite store.
func (m *Module) Close() error {
	if m.cancel != nil {
		m.cancel()
	}
	m.bg.Wait()
	if m.store != nil {
		return m.store.Close()
	}
	return nil
}
