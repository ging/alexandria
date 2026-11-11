// Package config loads the PoC server.json with the Appendix B fields (SQLite instead of PostgreSQL), validated against its own schema.
// The schema is not normative (Q16); relative file: paths resolve from the folder of server.json (DP-11).
package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
)

//go:embed server.schema.json
var schemaJSON []byte

// Schema returns the embedded server configuration schema.
func Schema() []byte { return schemaJSON }

// Issuer is one accepted token issuer (A2, A3).
type Issuer struct {
	Iss          string `json:"iss"`
	DiscoveryURL string `json:"discovery_url"`
}

// Server is server.json.
type Server struct {
	Listen  string `json:"listen"`
	Base    string `json:"base"`
	DID     string `json:"did"`
	Signing struct {
		Key                string `json:"key"`
		VerificationMethod string `json:"verification_method"`
	} `json:"signing"`
	RDF struct {
		Context string `json:"context"`
	} `json:"rdf"`
	Auth struct {
		Issuers    []Issuer `json:"issuers"`
		Audience   string   `json:"audience"`
		Algorithms []string `json:"algorithms"`
		AdminClaim struct {
			Claim string `json:"claim"`
			Value string `json:"value"`
		} `json:"admin_claim"`
		JWKSCache string `json:"jwks_cache"`
	} `json:"auth"`
	Vocab struct {
		MaxSize                int64    `json:"max_size"`
		ProxyTimeout           string   `json:"proxy_timeout"`
		ProxyAllow             []string `json:"proxy_allow"`
		AllowInsecureLocalhost bool     `json:"allow_insecure_localhost"`
	} `json:"vocab"`
	DIDResolver struct {
		AllowInsecureLocalhost bool `json:"allow_insecure_localhost"`
	} `json:"did_resolver"`
	Storage struct {
		Database string `json:"database"`
		Blobs    string `json:"blobs"`
	} `json:"storage"`
	TLS struct {
		Cert string `json:"cert"`
		Key  string `json:"key"`
	} `json:"tls"`
	ClockSkew string          `json:"clock_skew"`
	ClockFile string          `json:"clock_file"`
	Compiler  string          `json:"compiler"`
	VendorDir string          `json:"vendor_dir"`
	Lifecycle json.RawMessage `json:"lifecycle"`

	Defaults lifecycle.Params `json:"-"`
	Skew     timex.Duration   `json:"-"`
	JWKSTTL  timex.Duration   `json:"-"`
	Timeout  timex.Duration   `json:"-"`
}

// Load reads, validates and normalises a server.json, applying the Appendix B defaults.
func Load(path string) (Server, error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator on the command line.
	if err != nil {
		return Server{}, err
	}
	s, err := schemas.Compile(schemaJSON)
	if err != nil {
		return Server{}, err
	}
	if err := s.Validate(b); err != nil {
		return Server{}, fmt.Errorf("server.json: %w", err)
	}
	var c Server
	if err := json.Unmarshal(b, &c); err != nil {
		return Server{}, err
	}
	if c.Defaults, err = lifecycle.ParseOver(lifecycle.Defaults(), c.Lifecycle); err != nil {
		return Server{}, err
	}
	if errs := c.Defaults.Validate(); len(errs) > 0 {
		return Server{}, fmt.Errorf("server.json lifecycle: %w", errs[0])
	}
	c.applyDefaults()
	for _, d := range []struct {
		s   string
		dst *timex.Duration
	}{{c.ClockSkew, &c.Skew}, {c.Auth.JWKSCache, &c.JWKSTTL}, {c.Vocab.ProxyTimeout, &c.Timeout}} {
		if *d.dst, err = timex.ParseDuration(d.s); err != nil {
			return Server{}, fmt.Errorf("server.json: %w", err)
		}
	}
	dir := filepath.Dir(path)
	for _, p := range []*string{&c.Signing.Key, &c.Storage.Database, &c.Storage.Blobs, &c.ClockFile, &c.VendorDir, &c.TLS.Cert, &c.TLS.Key} {
		*p = resolve(dir, *p)
	}
	return c, nil
}

// applyDefaults fills the Appendix B defaults.
func (c *Server) applyDefaults() {
	set := func(p *string, v string) {
		if *p == "" {
			*p = v
		}
	}
	set(&c.RDF.Context, "referenced")
	set(&c.Auth.AdminClaim.Claim, "roles")
	set(&c.Auth.AdminClaim.Value, "vhub-admin")
	set(&c.Auth.JWKSCache, "PT10M")
	set(&c.Vocab.ProxyTimeout, "PT30S")
	set(&c.ClockSkew, "PT5S")
	set(&c.Compiler, "poc")
	if len(c.Auth.Algorithms) == 0 {
		c.Auth.Algorithms = []string{"RS256", "ES256", "EdDSA"}
	}
	if c.Vocab.MaxSize == 0 {
		c.Vocab.MaxSize = 10485760
	}
}

// resolve strips file: and makes a relative path absolute from dir.
func resolve(dir, p string) string {
	p = strings.TrimPrefix(strings.TrimPrefix(p, "file://"), "file:")
	if p != "" && !filepath.IsAbs(p) {
		return filepath.Join(dir, p)
	}
	return p
}

// Context returns the value of C (§3.4) for this server.
func (c Server) Context() string {
	if c.RDF.Context == "self-hosted" {
		return c.Base + "/pub/_vhub/v1/context.jsonld"
	}
	return "https://w3id.org/alexandria-vhub/v1"
}
