// Composition root of alexandria-server: loads configuration, wires internal/server.Module, and serves HTTP/HTTPS.
// Mounts /v1 and /pub and starts the listing and signing-key scheduler (REQ-HUB-008, REQ-HUB-015, DP-2, DP-8, DP-11, DP-14).
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server"
	"github.com/ging/alexandria/internal/server/config"
	"github.com/ging/alexandria/internal/telemetry"
)

// main parses flags and starts the alexandria-server process.
func main() {
	cfgPath := flag.String("config", "config/server.yaml", "path to server configuration (YAML or JSON)")
	clockFile := flag.String("clock-file", "", "read the current instant from this file (PoC and tests only)")
	logLevel := flag.String("log-level", "", "override observability.log_level (debug, info, warn, error)")
	logFormat := flag.String("log-format", "", "override observability.log_format (auto, json, text)")
	flag.Parse()
	if err := run(*cfgPath, *clockFile, *logLevel, *logFormat); err != nil {
		fmt.Fprintln(os.Stderr, "alexandria-server:", err)
		os.Exit(1)
	}
}

// run builds the server module and blocks until a termination signal.
func run(cfgPath, clockFile, logLevel, logFormat string) error {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return err
	}
	if clockFile != "" {
		cfg.ClockFile = clockFile
	}
	if logLevel != "" {
		cfg.Observability.LogLevel = logLevel
	}
	if logFormat != "" {
		cfg.Observability.LogFormat = logFormat
	}
	var clk timex.Clock = clock.System{}
	if cfg.ClockFile != "" {
		if clk, err = clock.NewFile(cfg.ClockFile); err != nil {
			return err
		}
	}
	svcName := cfg.Observability.ServiceName
	if svcName == "" {
		svcName = server.Name
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	rt, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:  svcName,
		Environment:  cfg.Observability.Environment,
		Level:        cfg.Observability.LogLevel,
		Format:       cfg.Observability.LogFormat,
		OTLPEndpoint: cfg.Observability.OTLPEndpoint,
		OTLPInsecure: cfg.Observability.OTLPInsecure,
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

	if cfg.TLS.Cert != "" || cfg.TLS.Key != "" {
		if err := ensureLabTLS(cfg.TLS.Cert, cfg.TLS.Key, cfg.Base, clk.Now().Time()); err != nil {
			rt.Logger.ErrorContext(ctx, "failed to prepare lab TLS certificate", logx.Err(err))
			return err
		}
	}
	mod, err := server.New(server.Deps{
		Config:     cfg,
		Clock:      clk,
		Logger:     rt.Logger,
		Tracer:     rt.Tracer,
		Propagator: rt.Propagator,
	})
	if err != nil {
		rt.Logger.ErrorContext(ctx, "failed to initialize server module", logx.Err(err))
		return err
	}
	defer func() { _ = mod.Close() }()
	if err := mod.Start(ctx); err != nil {
		return err
	}
	srv := &http.Server{
		Addr:              cfg.Listen,
		Handler:           mod.Handler(),
		ErrorLog:          rt.ErrorLog(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		rt.Logger.Info("shutting down server", slog.String("listen", cfg.Listen))
		sctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	rt.Logger.InfoContext(ctx, "server listening",
		slog.String("listen", cfg.Listen),
		slog.String("base", cfg.Base),
		slog.String("hub_did", string(mod.DID())),
		slog.Bool("tls_enabled", cfg.TLS.Cert != ""),
		slog.String("note", "PoC: not for production"),
	)
	serve := srv.ListenAndServe
	if cfg.TLS.Cert != "" {
		serve = func() error { return srv.ListenAndServeTLS(cfg.TLS.Cert, cfg.TLS.Key) }
	}
	if err := serve(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		rt.Logger.ErrorContext(ctx, "http server terminated with error", logx.Err(err))
		return err
	}
	rt.Logger.Info("server stopped")
	return nil
}

// ensureLabTLS generates a self-signed laboratory TLS certificate and key when either file is absent on disk (DP-14).
func ensureLabTLS(certPath, keyPath, baseURL string, now time.Time) error {
	if certPath == "" || keyPath == "" {
		return errors.New("tls.cert and tls.key must both be set when TLS is enabled")
	}
	_, certErr := os.Stat(certPath)
	_, keyErr := os.Stat(keyPath)
	if certErr == nil && keyErr == nil {
		return nil
	}
	if certErr != nil && !errors.Is(certErr, os.ErrNotExist) {
		return certErr
	}
	if keyErr != nil && !errors.Is(keyErr, os.ErrNotExist) {
		return keyErr
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o750); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o750); err != nil {
		return err
	}
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	ips := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	dns := []string{"localhost"}
	if u, err := url.Parse(baseURL); err == nil && u.Hostname() != "" {
		if ip := net.ParseIP(u.Hostname()); ip != nil {
			ips = append(ips, ip)
		} else if u.Hostname() != "localhost" {
			dns = append(dns, u.Hostname())
		}
	}
	tpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "alexandria-hub-lab"},
		IPAddresses:           ips,
		DNSNames:              dns,
		NotBefore:             now.Add(-24 * time.Hour),
		NotAfter:              now.Add(3650 * 24 * time.Hour),
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
	if err != nil {
		return err
	}
	kd, err := x509.MarshalPKCS8PrivateKey(k)
	if err != nil {
		return err
	}
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: kd}), 0o600); err != nil {
		return err
	}
	slog.Info("generated lab TLS certificate", slog.String("cert_path", certPath), slog.String("note", "PoC: not for production"))
	return nil
}
