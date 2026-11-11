// Package config loads the server configuration from JSON or YAML, supporting both Appendix B and the config.yaml section layout.
// Relative file: paths resolve from the directory of the configuration file (DP-11).
package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

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

// Server is the validated runtime configuration of alexandria-server.
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

// Endpoint models one HTTP transport address (public and internal) in the config.yaml layout.
type Endpoint struct {
	Protocol     string `json:"protocol"`
	URL          string `json:"url"`
	Port         string `json:"port,omitempty"`
	InternalPort string `json:"internal_port,omitempty"`
}

// NodeConfig is the config.yaml-inspired server configuration layout (common_config, auth_config, did_config, observability, vhub_config).
type NodeConfig struct {
	Common struct {
		Hosts struct {
			HTTP    Endpoint  `json:"http"`
			GRPC    *Endpoint `json:"grpc,omitempty"`
			GraphQL *Endpoint `json:"graphql,omitempty"`
		} `json:"hosts"`
		DB struct {
			DBType   string `json:"db_type,omitempty"`
			Database string `json:"database"`
			Blobs    string `json:"blobs"`
		} `json:"db"`
		Connection struct {
			IsLocal     bool `json:"is_local"`
			IsProd      bool `json:"is_prod"`
			HasTLSProxy bool `json:"has_tls_proxy"`
			TLS         struct {
				Cert string `json:"cert,omitempty"`
				Key  string `json:"key,omitempty"`
			} `json:"tls,omitempty"`
		} `json:"connection"`
	} `json:"common_config"`
	Auth struct {
		Enabled        *bool    `json:"enabled,omitempty"`
		Issuer         string   `json:"issuer,omitempty"`
		InternalIssuer string   `json:"internal_issuer,omitempty"`
		Issuers        []Issuer `json:"issuers,omitempty"`
		Audience       string   `json:"audience,omitempty"`
		Audiences      []string `json:"audiences,omitempty"`
		Algorithms     []string `json:"algorithms,omitempty"`
		RolesClaim     string   `json:"roles_claim,omitempty"`
		AdminRole      string   `json:"admin_role,omitempty"`
		JWKSRefresh    string   `json:"jwks_refresh,omitempty"`
	} `json:"auth_config"`
	DID struct {
		Type                   string `json:"type,omitempty"`
		DID                    string `json:"did,omitempty"`
		Domain                 string `json:"domain,omitempty"`
		SigningKey             string `json:"signing_key"`
		VerificationMethod     string `json:"verification_method,omitempty"`
		AllowInsecureLocalhost *bool  `json:"allow_insecure_localhost,omitempty"`
	} `json:"did_config"`
	Observability struct {
		LogLevel  string `json:"log_level,omitempty"`
		LogFormat string `json:"log_format,omitempty"`
		Metrics   *bool  `json:"metrics,omitempty"`
		Pprof     *bool  `json:"pprof,omitempty"`
		Port      string `json:"port,omitempty"`
	} `json:"observability"`
	VHub struct {
		RDFContext string          `json:"rdf_context,omitempty"`
		Compiler   string          `json:"compiler,omitempty"`
		VendorDir  string          `json:"vendor_dir,omitempty"`
		ClockSkew  string          `json:"clock_skew,omitempty"`
		ClockFile  string          `json:"clock_file,omitempty"`
		Lifecycle  json.RawMessage `json:"lifecycle,omitempty"`
		Vocab      struct {
			MaxSize                int64    `json:"max_size,omitempty"`
			ProxyTimeout           string   `json:"proxy_timeout,omitempty"`
			ProxyAllow             []string `json:"proxy_allow,omitempty"`
			AllowInsecureLocalhost *bool    `json:"allow_insecure_localhost,omitempty"`
		} `json:"vocab,omitempty"`
	} `json:"vhub_config"`
}

// Load reads, validates and normalises a server configuration file (JSON or YAML).
func Load(path string) (Server, error) {
	raw, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator on the command line.
	if err != nil {
		return Server{}, err
	}
	c, err := Parse(raw)
	if err != nil {
		return Server{}, err
	}
	dir := filepath.Dir(path)
	for _, p := range []*string{&c.Signing.Key, &c.Storage.Database, &c.Storage.Blobs, &c.ClockFile, &c.VendorDir, &c.TLS.Cert, &c.TLS.Key} {
		*p = resolve(dir, *p)
	}
	return c, nil
}

// Parse validates and normalises a JSON or YAML server configuration document.
func Parse(raw []byte) (Server, error) {
	b, err := toJSON(raw)
	if err != nil {
		return Server{}, fmt.Errorf("server config: %w", err)
	}
	if isNodeLayout(b) {
		if b, err = convertNodeConfig(b); err != nil {
			return Server{}, fmt.Errorf("server config: %w", err)
		}
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
	return c, nil
}

// toJSON converts YAML or JSON bytes to canonical JSON bytes.
func toJSON(raw []byte) ([]byte, error) {
	if json.Valid(raw) {
		return raw, nil
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return nil, fmt.Errorf("invalid JSON")
	}
	var v any
	if err := yaml.Unmarshal(raw, &v); err != nil {
		return nil, err
	}
	return json.Marshal(v)
}

// isNodeLayout reports whether the JSON object uses the config.yaml section layout.
func isNodeLayout(b []byte) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	_, hasCommon := m["common_config"]
	_, hasAuth := m["auth_config"]
	_, hasDID := m["did_config"]
	_, hasVHub := m["vhub_config"]
	return hasCommon || hasAuth || hasDID || hasVHub
}

// convertNodeConfig translates a NodeConfig document into the canonical Appendix B JSON format.
func convertNodeConfig(b []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var nc NodeConfig
	if err := dec.Decode(&nc); err != nil {
		return nil, err
	}
	out := map[string]any{}
	httpEp := nc.Common.Hosts.HTTP
	privPort := httpEp.InternalPort
	if privPort == "" {
		privPort = httpEp.Port
	}
	if httpEp.URL != "" && privPort != "" {
		out["listen"] = net.JoinHostPort(httpEp.URL, privPort)
	}
	if httpEp.Protocol != "" && httpEp.URL != "" {
		auth := httpEp.URL
		if httpEp.Port != "" {
			auth = net.JoinHostPort(httpEp.URL, httpEp.Port)
		}
		out["base"] = httpEp.Protocol + "://" + auth
	}
	if nc.DID.DID != "" {
		out["did"] = nc.DID.DID
	} else if strings.EqualFold(nc.DID.Type, "web") && nc.DID.Domain != "" {
		out["did"] = "did:web:" + nc.DID.Domain
	}
	signing := map[string]any{}
	if nc.DID.SigningKey != "" {
		signing["key"] = nc.DID.SigningKey
	}
	if nc.DID.VerificationMethod != "" {
		signing["verification_method"] = nc.DID.VerificationMethod
	}
	if len(signing) > 0 {
		out["signing"] = signing
	}
	if nc.VHub.RDFContext != "" {
		out["rdf"] = map[string]any{"context": nc.VHub.RDFContext}
	}
	issuers := nc.Auth.Issuers
	if len(issuers) == 0 && nc.Auth.Issuer != "" {
		disc := nc.Auth.InternalIssuer
		if disc == "" {
			disc = nc.Auth.Issuer
		}
		issuers = []Issuer{{Iss: nc.Auth.Issuer, DiscoveryURL: strings.TrimSuffix(disc, "/") + "/.well-known/openid-configuration"}}
	}
	aud := nc.Auth.Audience
	if aud == "" && len(nc.Auth.Audiences) > 0 {
		aud = nc.Auth.Audiences[0]
	}
	authMap := map[string]any{"issuers": issuers, "audience": aud}
	if len(nc.Auth.Algorithms) > 0 {
		authMap["algorithms"] = nc.Auth.Algorithms
	}
	if nc.Auth.RolesClaim != "" || nc.Auth.AdminRole != "" {
		claim, val := nc.Auth.RolesClaim, nc.Auth.AdminRole
		if claim == "" {
			claim = "roles"
		}
		if val == "" {
			val = "vhub-admin"
		}
		authMap["admin_claim"] = map[string]string{"claim": claim, "value": val}
	}
	if nc.Auth.JWKSRefresh != "" {
		authMap["jwks_cache"] = nc.Auth.JWKSRefresh
	}
	out["auth"] = authMap
	vocab := map[string]any{}
	if nc.VHub.Vocab.MaxSize != 0 {
		vocab["max_size"] = nc.VHub.Vocab.MaxSize
	}
	if nc.VHub.Vocab.ProxyTimeout != "" {
		vocab["proxy_timeout"] = nc.VHub.Vocab.ProxyTimeout
	}
	if len(nc.VHub.Vocab.ProxyAllow) > 0 {
		vocab["proxy_allow"] = nc.VHub.Vocab.ProxyAllow
	}
	if nc.VHub.Vocab.AllowInsecureLocalhost != nil {
		vocab["allow_insecure_localhost"] = *nc.VHub.Vocab.AllowInsecureLocalhost
	} else if nc.Common.Connection.IsLocal {
		vocab["allow_insecure_localhost"] = true
	}
	if len(vocab) > 0 {
		out["vocab"] = vocab
	}
	if nc.DID.AllowInsecureLocalhost != nil {
		out["did_resolver"] = map[string]bool{"allow_insecure_localhost": *nc.DID.AllowInsecureLocalhost}
	} else if nc.Common.Connection.IsLocal {
		out["did_resolver"] = map[string]bool{"allow_insecure_localhost": true}
	}
	if nc.Common.DB.Database != "" || nc.Common.DB.Blobs != "" {
		out["storage"] = map[string]string{"database": nc.Common.DB.Database, "blobs": nc.Common.DB.Blobs}
	}
	if !nc.Common.Connection.HasTLSProxy && (nc.Common.Connection.TLS.Cert != "" || nc.Common.Connection.TLS.Key != "") {
		out["tls"] = map[string]string{"cert": nc.Common.Connection.TLS.Cert, "key": nc.Common.Connection.TLS.Key}
	}
	if nc.VHub.ClockSkew != "" {
		out["clock_skew"] = nc.VHub.ClockSkew
	}
	if nc.VHub.ClockFile != "" {
		out["clock_file"] = nc.VHub.ClockFile
	}
	if nc.VHub.Compiler != "" {
		out["compiler"] = nc.VHub.Compiler
	}
	if nc.VHub.VendorDir != "" {
		out["vendor_dir"] = nc.VHub.VendorDir
	}
	if len(nc.VHub.Lifecycle) > 0 {
		out["lifecycle"] = nc.VHub.Lifecycle
	}
	return json.Marshal(out)
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
