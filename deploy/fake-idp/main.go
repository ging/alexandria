// fake-idp is the PoC identity provider: OIDC Discovery, JWKS, an interactive authorization-code flow with PKCE, and a CLI that prints at+jwt tokens.
// It replaces a real IdP in the demo and the e2e so the hub validates real tokens with jwt_jwks (DP-4, PRE-07).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/telemetry"
	"github.com/ging/alexandria/internal/testkit"
)

// main dispatches the serve and token subcommands.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: fake-idp serve|token [flags]")
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "serve":
		err = serve(os.Args[2:])
	case "token":
		err = token(os.Args[2:])
	default:
		err = errors.New("unknown subcommand " + os.Args[1])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "fake-idp:", err)
		os.Exit(1)
	}
}

// common declares the flags shared by both subcommands.
func common(fs *flag.FlagSet) (*string, *string) {
	return fs.String("issuer", "http://127.0.0.1:9400", "issuer URL"), fs.String("key", "fake-idp.key", "ES256 key file (generated if absent)")
}

// serve runs discovery, JWKS and the interactive authorization-code flow; --clock-file aligns issued tokens with a file clock.
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	iss, key := common(fs)
	addr := fs.String("addr", "127.0.0.1:9400", "listen address")
	ttl := fs.Duration("ttl", time.Hour, "lifetime of tokens issued by /token")
	clockFile := fs.String("clock-file", "", "read the issuing instant from this file")
	logLevel := fs.String("log-level", "info", "log level (debug, info, warn, error)")
	logFormat := fs.String("log-format", "auto", "log format (auto, json, text)")
	_ = fs.Parse(args)

	ctx := context.Background()
	rt, err := telemetry.Setup(ctx, telemetry.Config{ServiceName: "alexandria-fake-idp", Level: *logLevel, Format: *logFormat, SetGlobal: true})
	if err != nil {
		return err
	}
	defer func() { _ = rt.Shutdown(ctx) }()

	idp, err := testkit.LoadIDP(*iss, *key)
	if err != nil {
		return err
	}
	now := time.Now
	if *clockFile != "" {
		now = func() time.Time {
			if t, err := fileInstant(*clockFile); err == nil {
				return t
			}
			return time.Now()
		}
	}
	rt.Logger.Info("fake-idp listening", slog.String("issuer", *iss), slog.String("listen", *addr), slog.String("sign_in", *iss+"/authorize"), slog.String("note", "PoC: not for production"))
	handler := telemetry.HTTPMiddleware(telemetry.HTTPOptions{
		Logger:     rt.Logger,
		Tracer:     rt.Tracer,
		Propagator: rt.Propagator,
		Component:  "http.idp",
	}, testkit.NewOAuth(idp, now, *ttl).Handler())
	srv := &http.Server{Addr: *addr, Handler: handler, ErrorLog: rt.ErrorLog(), ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
}

// fileInstant reads the current instant from a file clock.
func fileInstant(path string) (time.Time, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- demo clock file.
	if err != nil {
		return time.Time{}, err
	}
	ts, err := timex.ParseInstant(strings.TrimSpace(string(b)))
	if err != nil {
		return time.Time{}, err
	}
	return ts.Time(), nil
}

// token prints one access token; --clock-file aligns iat, nbf and exp with a file clock.
func token(args []string) error {
	fs := flag.NewFlagSet("token", flag.ExitOnError)
	iss, key := common(fs)
	sub := fs.String("sub", "", "subject")
	admin := fs.Bool("admin", false, "add the vhub-admin role")
	aud := fs.String("aud", "alexandria-vhub", "audience")
	ttl := fs.Duration("ttl", time.Hour, "token lifetime")
	clockFile := fs.String("clock-file", "", "read the issuing instant from this file")
	_ = fs.Parse(args)
	if *sub == "" {
		return errors.New("--sub is required")
	}
	idp, err := testkit.LoadIDP(*iss, *key)
	if err != nil {
		return err
	}
	now := time.Now()
	if *clockFile != "" {
		if now, err = fileInstant(*clockFile); err != nil {
			return err
		}
	}
	tok, err := idp.Mint(testkit.Token{Sub: *sub, Admin: *admin, Audience: *aud, Now: now, TTL: *ttl})
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}
