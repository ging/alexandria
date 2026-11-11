// fake-idp is the PoC identity provider: it serves OIDC Discovery and a JWKS, and prints at+jwt access tokens signed with ES256.
// It replaces a real IdP in the demo and the e2e so the hub validates real tokens with jwt_jwks (DP-4, PRE-07).
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ging/alexandria/internal/core/timex"
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

// serve runs the discovery and JWKS endpoints.
func serve(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	iss, key := common(fs)
	addr := fs.String("addr", "127.0.0.1:9400", "listen address")
	_ = fs.Parse(args)
	idp, err := testkit.LoadIDP(*iss, *key)
	if err != nil {
		return err
	}
	log.Printf("fake-idp %s on %s (PoC: not for production)", *iss, *addr)
	srv := &http.Server{Addr: *addr, Handler: idp.Handler(), ReadHeaderTimeout: 5 * time.Second}
	return srv.ListenAndServe()
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
	now := timex.FromTime(time.Now())
	if *clockFile != "" {
		b, err := os.ReadFile(*clockFile) // #nosec G304 -- demo clock file.
		if err != nil {
			return err
		}
		if now, err = timex.ParseInstant(strings.TrimSpace(string(b))); err != nil {
			return err
		}
	}
	tok, err := idp.Mint(testkit.Token{Sub: *sub, Admin: *admin, Audience: *aud, Now: now.Time(), TTL: *ttl})
	if err != nil {
		return err
	}
	fmt.Println(tok)
	return nil
}
