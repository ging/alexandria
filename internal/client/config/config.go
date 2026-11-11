// Package config loads the normative alexandria-client configuration (Appendix A.3) from JSON or JSON-model YAML and checks CFG-1.
// CFG-2 (the rules seed) is checked by Rules.Init with an empty store; the first failing code is reported (§4.3.1, REQ-CLI-001, REQ-DOC-003).
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
)

// DefaultContext is the default value of trust.context (§3.4).
const DefaultContext = "https://w3id.org/alexandria-vhub/v1"

// Trust is the trust section.
type Trust struct {
	Hub       string `json:"hub"`
	Namespace string `json:"namespace"`
	HubDID    string `json:"hub_did"`
	Context   string `json:"context,omitempty"`
	DIDTTL    string `json:"did_ttl,omitempty"`
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
	Trust     Trust               `json:"trust"`
	Upstreams Upstreams           `json:"upstreams"`
	Listeners map[string]Listener `json:"listeners"`
	Rules     struct {
		Seed string `json:"seed,omitempty"`
	} `json:"rules"`
	DefaultAction    string `json:"default_action,omitempty"`
	UnknownAgreement string `json:"unknown_agreement,omitempty"`
	Audit            struct {
		Window int `json:"window,omitempty"`
	} `json:"audit"`
	Retention struct {
		GCInterval string `json:"gc_interval,omitempty"`
	} `json:"retention"`
	Events struct {
		Sink string `json:"sink"`
	} `json:"events"`

	DIDTTL     timex.Duration `json:"-"`
	GCInterval timex.Duration `json:"-"`
}

// Load reads a JSON or YAML configuration file.
func Load(path string) (*Config, *codes.Error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator on the command line.
	if err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	if ext := strings.ToLower(filepath.Ext(path)); ext == ".yaml" || ext == ".yml" {
		var v any
		if err := yaml.Unmarshal(b, &v); err != nil {
			return nil, codes.New(codes.ConfigSchema, "YAML: %v", err)
		}
		if b, err = json.Marshal(v); err != nil {
			return nil, codes.New(codes.ConfigSchema, "YAML is not in the JSON model: %v", err)
		}
	}
	return Parse(b)
}

// Parse applies CFG-1 and the defaults of §4.3.1.
func Parse(b []byte) (*Config, *codes.Error) {
	if err := schemas.Validate(schemas.Config, b); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	c.applyDefaults()
	var err error
	if c.DIDTTL, err = timex.ParseDuration(c.Trust.DIDTTL); err != nil {
		return nil, codes.New(codes.ConfigSchema, "did_ttl: %v", err)
	}
	if c.GCInterval, err = timex.ParseDuration(c.Retention.GCInterval); err != nil {
		return nil, codes.New(codes.ConfigSchema, "gc_interval: %v", err)
	}
	return &c, nil
}

// applyDefaults fills the defaults of §4.3.1.
func (c *Config) applyDefaults() {
	set := func(p *string, v string) {
		if *p == "" {
			*p = v
		}
	}
	set(&c.Trust.Context, DefaultContext)
	set(&c.Trust.DIDTTL, "PT6H")
	set(&c.DefaultAction, "forward")
	set(&c.UnknownAgreement, "forward")
	set(&c.Retention.GCInterval, "P1D")
	if c.Audit.Window == 0 {
		c.Audit.Window = 100
	}
}

// NamespaceIRI is the expected namespace IRI {trust.hub}/v1/namespaces/{trust.namespace}.
func (c *Config) NamespaceIRI() string {
	return strings.TrimSuffix(c.Trust.Hub, "/") + "/v1/namespaces/" + c.Trust.Namespace
}
