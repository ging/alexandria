// Loader of the normative vhub-client configuration (Appendix A.3) with the CFG-1..CFG-5 startup invariants in order.
// JSON only in the PoC; YAML is not supported (agent decision). The first failing code is reported (§4.3, REQ-CLI-001, REQ-DOC-003).
package config

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
)

// Trust is the trust section.
type Trust struct {
	HubDID      string `json:"hub_did"`
	Hub         string `json:"hub"`
	Credentials string `json:"credentials"`
	DIDTTL      string `json:"did_ttl,omitempty"`
}

// Upstreams is the upstreams section.
type Upstreams struct {
	DSP        string `json:"dsp"`
	Management string `json:"management,omitempty"`
}

// Listener is one listeners entry.
type Listener struct {
	Bind string `json:"bind"`
}

// Config is the client configuration document.
type Config struct {
	Space            string              `json:"space"`
	Trust            Trust               `json:"trust"`
	Upstreams        Upstreams           `json:"upstreams"`
	Listeners        map[string]Listener `json:"listeners"`
	Routes           []domain.Route      `json:"routes"`
	DefaultAction    string              `json:"default_action,omitempty"`
	UnknownAgreement string              `json:"unknown_agreement,omitempty"`
	Audit            struct {
		Window int `json:"window,omitempty"`
	} `json:"audit"`
	Events struct {
		Sink string `json:"sink"`
	} `json:"events"`
	// Token is the member bearer token read from trust.credentials (DP-3).
	Token string `json:"-"`
	// DIDTTL is trust.did_ttl parsed with its default.
	DIDTTLValue timex.Duration `json:"-"`
}

// credentials is the PoC credentials file: {"token": "<member bearer token>"}.
type credentials struct {
	Token string `json:"token"`
}

// Load reads a configuration and checks CFG-1..CFG-5, returning the first failure.
func Load(path string) (*Config, *codes.Error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	return Parse(b)
}

// Parse checks CFG-1..CFG-5 on a configuration document.
func Parse(b []byte) (*Config, *codes.Error) {
	if err := schemas.Validate(schemas.Config, b); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	if e := domain.CheckOverlap(c.Routes); e != nil {
		return nil, e
	}
	if e := domain.CheckGateMessage(c.Routes); e != nil {
		return nil, e
	}
	tok, e := readCredentials(c.Trust.Credentials)
	if e != nil {
		return nil, e
	}
	c.Token = tok
	if e := domain.CheckUpstream(c.Routes, c.Upstreams.Management != ""); e != nil {
		return nil, e
	}
	c.applyDefaults()
	return &c, nil
}

// readCredentials is CFG-4.
func readCredentials(uri string) (string, *codes.Error) {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "file" {
		return "", codes.New(codes.ConfigCredentialsUnreadable, "%s is not a file: URI", uri)
	}
	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", codes.New(codes.ConfigCredentialsUnreadable, "%v", err)
	}
	var c credentials
	if json.Unmarshal(b, &c) != nil || strings.TrimSpace(c.Token) == "" {
		return "", codes.New(codes.ConfigCredentialsUnreadable, "%s has no token", p)
	}
	return c.Token, nil
}

// applyDefaults fills the defaults of §4.3.
func (c *Config) applyDefaults() {
	if c.DefaultAction == "" {
		c.DefaultAction = "forward"
	}
	if c.UnknownAgreement == "" {
		c.UnknownAgreement = "forward"
	}
	if c.Audit.Window == 0 {
		c.Audit.Window = 100
	}
	c.DIDTTLValue = 6 * timex.Hour
	if c.Trust.DIDTTL != "" {
		if d, err := timex.ParseDuration(c.Trust.DIDTTL); err == nil {
			c.DIDTTLValue = d
		}
	}
}

// Gates returns the most restrictive configured mode per gate, for /status.
func (c *Config) Gates() map[domain.Gate]domain.Mode {
	rank := map[domain.Mode]int{domain.Observe: 0, domain.Shadow: 1, domain.Enforce: 2}
	out := map[domain.Gate]domain.Mode{}
	for _, r := range c.Routes {
		for _, g := range r.Gates {
			if m, ok := out[g]; !ok || rank[r.EffectiveMode()] > rank[m] {
				out[g] = r.EffectiveMode()
			}
		}
	}
	return out
}
