// Package server wires the vhub-server bounded context and manages its lifecycle.
// It connects persistence, signing, static or external authentication, and HTTP handlers (DP-2, DP-3, DP-4, DP-8, DP-12, REQ-HUB-008).
package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/dsl"
	"github.com/ging/alexandria/internal/server/adapters/auth_static"
	"github.com/ging/alexandria/internal/server/adapters/blob_fs"
	"github.com/ging/alexandria/internal/server/adapters/compiler_dsl"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/adapters/http_did"
	"github.com/ging/alexandria/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/internal/server/adapters/http_v1"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/adapters/scheduler"
	"github.com/ging/alexandria/internal/server/adapters/scitt_fake"
	"github.com/ging/alexandria/internal/server/adapters/signer_file"
	"github.com/ging/alexandria/internal/server/adapters/sqlite"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/config"
)

// Name identifies the vhub-server bounded context in logs and startup reports.
const Name = "vhub-server"

// Deps groups the configuration and optional port overrides for the server module.
type Deps struct {
	Config config.Server
	Clock  timex.Clock
	Signer app.Signer
	Auth   http_v1.Authenticator
}

// Module is the assembled vhub-server context with its HTTP surface and background scheduler.
type Module struct {
	hub     *app.Hub
	handler http.Handler
	store   *sqlite.Store
	clock   timex.Clock
	did     string
	listen  string
	bg      sync.WaitGroup
	cancel  context.CancelFunc
}

// New assembles the vhub-server module from its configuration and dependencies.
func New(deps Deps) (*Module, error) {
	cfg := deps.Config
	for _, p := range []string{cfg.SQLitePath, cfg.SigningKeyPath} {
		if p != "" {
			if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
				return nil, err
			}
		}
	}
	store, err := sqlite.Open(cfg.SQLitePath)
	if err != nil {
		return nil, err
	}
	blobs, err := blob_fs.New(cfg.BlobDir)
	if err != nil {
		_ = store.Close()
		return nil, err
	}
	signer := deps.Signer
	if signer == nil {
		signer, err = signer_file.Load(cfg.SigningKeyPath, did.DID(cfg.DID))
		if err != nil {
			_ = store.Close()
			return nil, err
		}
	}
	auth := deps.Auth
	if auth == nil {
		auth, err = auth_static.Load(cfg.TokensPath)
		if err != nil {
			_ = store.Close()
			return nil, err
		}
	}
	hub := app.New(app.Deps{
		Drafts:     store.Drafts(),
		Candidates: store.Candidates(),
		Releases:   store.Releases(),
		Listings:   store.Listings(),
		Blobs:      blobs,
		Log:        scitt_fake.Log{Signer: signer, Clock: deps.Clock},
		Signer:     signer,
		Compiler:   profileCompiler(cfg),
		Events:     store.Events(),
		Clock:      deps.Clock,
		DID:        did.DID(cfg.DID),
		Defaults:   cfg.Defaults,
	})
	mux := http.NewServeMux()
	http_v1.Handler{Hub: hub, Auth: auth}.Register(mux)
	http_pub.Handler{Pub: hub.Publication}.Register(mux)
	http_did.Handler{Listing: hub.Listing}.Register(mux)
	return &Module{
		hub:     hub,
		handler: httpx.WithDate(deps.Clock, mux),
		store:   store,
		clock:   deps.Clock,
		did:     cfg.DID,
		listen:  cfg.Listen,
	}, nil
}

// profileCompiler selects the draft compiler: the YAML language (dsl) or the PoC JSON drafts (poc, the default).
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

// Handler returns the HTTP handler mounting /v1, /pub, and /.well-known/did.json.
func (m *Module) Handler() http.Handler { return m.handler }

// Describe returns the hub DID for startup reporting.
func (m *Module) Describe() (string, bool) { return m.did, m.did != "" }

// Start launches the periodic listing re-issuance scheduler in the background.
func (m *Module) Start(ctx context.Context) error {
	ctx, m.cancel = context.WithCancel(ctx)
	m.bg.Add(1)
	go func() {
		defer m.bg.Done()
		scheduler.Run(ctx, m.hub.Listing, m.clock, time.Second)
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
