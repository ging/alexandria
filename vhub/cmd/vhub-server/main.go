// Composition root of vhub-server: loads server.json, builds the adapters, wires the use cases and serves HTTP.
// Mounts /v1, /pub and /.well-known/did.json and starts the listing scheduler (DP-2, DP-3, DP-4, DP-8, DP-9, DP-12).
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
	"path/filepath"
	"syscall"
	"time"

	"github.com/ging/alexandria/vhub/internal/clock"
	"github.com/ging/alexandria/vhub/internal/core/did"
	"github.com/ging/alexandria/vhub/internal/core/timex"
	"github.com/ging/alexandria/vhub/internal/server/adapters/auth_static"
	"github.com/ging/alexandria/vhub/internal/server/adapters/blob_fs"
	"github.com/ging/alexandria/vhub/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/vhub/internal/server/adapters/http_did"
	"github.com/ging/alexandria/vhub/internal/server/adapters/http_pub"
	"github.com/ging/alexandria/vhub/internal/server/adapters/http_v1"
	"github.com/ging/alexandria/vhub/internal/server/adapters/httpx"
	"github.com/ging/alexandria/vhub/internal/server/adapters/scheduler"
	"github.com/ging/alexandria/vhub/internal/server/adapters/scitt_fake"
	"github.com/ging/alexandria/vhub/internal/server/adapters/signer_file"
	"github.com/ging/alexandria/vhub/internal/server/adapters/sqlite"
	"github.com/ging/alexandria/vhub/internal/server/app"
	"github.com/ging/alexandria/vhub/internal/server/config"
)

func main() {
	cfgPath := flag.String("config", "server.json", "path to server.json")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	flag.Parse()
	if err := run(*cfgPath, *clockFile); err != nil {
		fmt.Fprintln(os.Stderr, "vhub-server:", err)
		os.Exit(1)
	}
}

// run builds the server and blocks until a termination signal.
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
	for _, p := range []string{cfg.SQLitePath, cfg.SigningKeyPath} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
	}
	store, err := sqlite.Open(cfg.SQLitePath)
	if err != nil {
		return err
	}
	defer store.Close()
	blobs, err := blob_fs.New(cfg.BlobDir)
	if err != nil {
		return err
	}
	signer, err := signer_file.Load(cfg.SigningKeyPath, did.DID(cfg.DID))
	if err != nil {
		return err
	}
	tokens, err := auth_static.Load(cfg.TokensPath)
	if err != nil {
		return err
	}
	hub := app.New(app.Deps{
		Drafts: store.Drafts(), Candidates: store.Candidates(), Releases: store.Releases(), Listings: store.Listings(),
		Blobs: blobs, Log: scitt_fake.Log{Signer: signer, Clock: clk}, Signer: signer,
		Compiler: compiler_poc.Compiler{Defaults: cfg.Defaults}, Events: store.Events(), Clock: clk,
		DID: did.DID(cfg.DID), Defaults: cfg.Defaults,
	})
	mux := http.NewServeMux()
	http_v1.Handler{Hub: hub, Auth: tokens}.Register(mux)
	http_pub.Handler{Pub: hub.Publication}.Register(mux)
	http_did.Handler{Listing: hub.Listing}.Register(mux)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go scheduler.Run(ctx, hub.Listing, clk, time.Second)
	srv := &http.Server{Addr: cfg.Listen, Handler: httpx.WithDate(clk, mux), ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()
	log.Printf("vhub-server listening on %s as %s (PoC: not for production)", cfg.Listen, cfg.DID)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
