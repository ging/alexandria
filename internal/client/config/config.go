// Package config loads the alexandria-client configuration from JSON or YAML, supporting both Appendix A.3 and the config.yaml section layout.
// CFG-2 (the rules seed) is checked by Rules.Init with an empty store; the first failing code is reported (§4.3.1, REQ-CLI-001, REQ-DOC-003).
package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/telemetry"
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

// LogConfig holds structured logging and OpenTelemetry export settings extracted from observability.
type LogConfig struct {
	Level        string
	Format       string
	OTLPEndpoint string
	OTLPInsecure bool
	ServiceName  string
	Environment  string
}

// Deployment modes: the client forwards traffic itself, or only answers the checks of an external proxy.
const (
	ModeProxy     = "proxy"
	ModeValidator = "validator"
)

// Config is the client configuration document.
type Config struct {
	Mode      string              `json:"mode,omitempty"`
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
	HubCA      string         `json:"-"`
	Log        LogConfig      `json:"-"`
}

// Endpoint models one HTTP transport address (public and internal) in the config.yaml layout.
type Endpoint struct {
	Protocol     string `json:"protocol"`
	URL          string `json:"url"`
	Port         string `json:"port,omitempty"`
	InternalPort string `json:"internal_port,omitempty"`
}

// NodeConfig is the config.yaml-inspired client configuration layout (common_config, did_config, observability, client_config).
type NodeConfig struct {
	Common struct {
		Hosts struct {
			HTTP    Endpoint  `json:"http"`
			GRPC    *Endpoint `json:"grpc,omitempty"`
			GraphQL *Endpoint `json:"graphql,omitempty"`
		} `json:"hosts"`
		Connection struct {
			IsLocal     bool `json:"is_local"`
			IsProd      bool `json:"is_prod"`
			HasTLSProxy bool `json:"has_tls_proxy"`
			TLS         struct {
				CA string `json:"ca,omitempty"`
			} `json:"tls,omitempty"`
		} `json:"connection"`
	} `json:"common_config"`
	DID struct {
		Type   string `json:"type,omitempty"`
		HubDID string `json:"hub_did,omitempty"`
		DIDTTL string `json:"did_ttl,omitempty"`
	} `json:"did_config"`
	Observability struct {
		LogLevel     string `json:"log_level,omitempty"`
		LogFormat    string `json:"log_format,omitempty"`
		OTLPEndpoint string `json:"otlp_endpoint,omitempty"`
		OTLPInsecure *bool  `json:"otlp_insecure,omitempty"`
		ServiceName  string `json:"service_name,omitempty"`
		Metrics      *bool  `json:"metrics,omitempty"`
		Pprof        *bool  `json:"pprof,omitempty"`
		Port         string `json:"port,omitempty"`
		EventsSink   string `json:"events_sink,omitempty"`
		AuditWindow  int    `json:"audit_window,omitempty"`
	} `json:"observability"`
	Client struct {
		ClassID   string              `json:"class_id,omitempty"`
		Display   *string             `json:"display,omitempty"`
		Mode      string              `json:"mode,omitempty"`
		Trust     Trust               `json:"trust"`
		Upstreams Upstreams           `json:"upstreams"`
		Listeners map[string]Listener `json:"listeners,omitempty"`
		Rules     struct {
			Seed string `json:"seed,omitempty"`
		} `json:"rules,omitempty"`
		DefaultAction    string `json:"default_action,omitempty"`
		UnknownAgreement string `json:"unknown_agreement,omitempty"`
		Audit            struct {
			Window int `json:"window,omitempty"`
		} `json:"audit,omitempty"`
		Retention struct {
			GCInterval string `json:"gc_interval,omitempty"`
		} `json:"retention,omitempty"`
		Events struct {
			Sink string `json:"sink,omitempty"`
		} `json:"events,omitempty"`
	} `json:"client_config"`
}

// Load reads a JSON or YAML configuration file.
func Load(path string) (*Config, *codes.Error) {
	b, err := os.ReadFile(path) // #nosec G304 -- path chosen by the operator on the command line.
	if err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	c, cerr := Parse(b)
	if cerr != nil {
		return nil, cerr
	}
	if c.HubCA != "" {
		p := strings.TrimPrefix(strings.TrimPrefix(c.HubCA, "file://"), "file:")
		if p != "" && !filepath.IsAbs(p) {
			p = filepath.Join(filepath.Dir(path), p)
		}
		c.HubCA = p
	}
	return c, nil
}

// Parse applies CFG-1 and the defaults of §4.3.1 to a JSON or YAML configuration document.
func Parse(raw []byte) (*Config, *codes.Error) {
	b, err := toJSON(raw)
	if err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	var hubCA string
	var logCfg LogConfig
	if isNodeLayout(b) {
		if b, hubCA, logCfg, err = convertNodeConfig(b); err != nil {
			return nil, codes.New(codes.ConfigSchema, "%v", err)
		}
	}
	if err := schemas.Validate(schemas.Config, b); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, codes.New(codes.ConfigSchema, "%v", err)
	}
	c.HubCA = hubCA
	c.Log = logCfg
	c.applyDefaults()
	if _, err := telemetry.ParseLevel(c.Log.Level); err != nil {
		return nil, codes.New(codes.ConfigSchema, "log_level: %v", err)
	}
	if err := telemetry.ValidateFormat(c.Log.Format); err != nil {
		return nil, codes.New(codes.ConfigSchema, "log_format: %v", err)
	}
	if c.DIDTTL, err = timex.ParseDuration(c.Trust.DIDTTL); err != nil {
		return nil, codes.New(codes.ConfigSchema, "did_ttl: %v", err)
	}
	if c.GCInterval, err = timex.ParseDuration(c.Retention.GCInterval); err != nil {
		return nil, codes.New(codes.ConfigSchema, "gc_interval: %v", err)
	}
	return &c, nil
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
		return nil, fmt.Errorf("YAML: %w", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("YAML is not in the JSON model: %w", err)
	}
	return b, nil
}

// isNodeLayout reports whether the JSON object uses the config.yaml section layout.
func isNodeLayout(b []byte) bool {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(b, &m); err != nil {
		return false
	}
	_, hasCommon := m["common_config"]
	_, hasDID := m["did_config"]
	_, hasClient := m["client_config"]
	return hasCommon || hasDID || hasClient
}

// convertNodeConfig translates a NodeConfig document into the canonical Appendix A.3 JSON format.
func convertNodeConfig(b []byte) ([]byte, string, LogConfig, error) {
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var nc NodeConfig
	if err := dec.Decode(&nc); err != nil {
		return nil, "", LogConfig{}, err
	}
	trust := nc.Client.Trust
	if trust.HubDID == "" && nc.DID.HubDID != "" {
		trust.HubDID = nc.DID.HubDID
	}
	if trust.DIDTTL == "" && nc.DID.DIDTTL != "" {
		trust.DIDTTL = nc.DID.DIDTTL
	}
	listeners := map[string]Listener{}
	for k, v := range nc.Client.Listeners {
		listeners[k] = v
	}
	httpEp := nc.Common.Hosts.HTTP
	privPort := httpEp.InternalPort
	if privPort == "" {
		privPort = httpEp.Port
	}
	if _, ok := listeners["dsp-ingress"]; !ok && httpEp.URL != "" && privPort != "" {
		listeners["dsp-ingress"] = Listener{Bind: net.JoinHostPort(httpEp.URL, privPort)}
	}
	if _, ok := listeners["admin"]; !ok && nc.Observability.Port != "" {
		host := httpEp.URL
		if host == "" {
			host = "127.0.0.1"
		}
		listeners["admin"] = Listener{Bind: net.JoinHostPort(host, nc.Observability.Port)}
	}
	out := map[string]any{
		"trust":     trust,
		"listeners": listeners,
	}
	if nc.Client.Mode != "" {
		out["mode"] = nc.Client.Mode
	}
	if nc.Client.Mode != ModeValidator || nc.Client.Upstreams != (Upstreams{}) {
		out["upstreams"] = nc.Client.Upstreams
	}
	if nc.Client.Rules.Seed != "" {
		out["rules"] = map[string]string{"seed": nc.Client.Rules.Seed}
	}
	if nc.Client.DefaultAction != "" {
		out["default_action"] = nc.Client.DefaultAction
	}
	if nc.Client.UnknownAgreement != "" {
		out["unknown_agreement"] = nc.Client.UnknownAgreement
	}
	window := nc.Client.Audit.Window
	if window == 0 {
		window = nc.Observability.AuditWindow
	}
	if window != 0 {
		out["audit"] = map[string]int{"window": window}
	}
	if nc.Client.Retention.GCInterval != "" {
		out["retention"] = map[string]string{"gc_interval": nc.Client.Retention.GCInterval}
	}
	sink := nc.Client.Events.Sink
	if sink == "" {
		sink = nc.Observability.EventsSink
	}
	out["events"] = map[string]string{"sink": sink}
	logCfg := LogConfig{
		Level:        nc.Observability.LogLevel,
		Format:       nc.Observability.LogFormat,
		OTLPEndpoint: nc.Observability.OTLPEndpoint,
		ServiceName:  nc.Observability.ServiceName,
	}
	if nc.Observability.OTLPInsecure != nil {
		logCfg.OTLPInsecure = *nc.Observability.OTLPInsecure
	}
	if nc.Common.Connection.IsProd {
		logCfg.Environment = "production"
	}
	rawOut, err := json.Marshal(out)
	return rawOut, nc.Common.Connection.TLS.CA, logCfg, err
}

// applyDefaults fills the defaults of §4.3.1.
func (c *Config) applyDefaults() {
	set := func(p *string, v string) {
		if *p == "" {
			*p = v
		}
	}
	set(&c.Mode, ModeProxy)
	set(&c.Trust.Context, DefaultContext)
	set(&c.Trust.DIDTTL, "PT6H")
	set(&c.DefaultAction, "forward")
	set(&c.UnknownAgreement, "forward")
	set(&c.Retention.GCInterval, "P1D")
	set(&c.Log.Level, "info")
	set(&c.Log.Format, "auto")
	if c.Audit.Window == 0 {
		c.Audit.Window = 100
	}
}

// Validator reports whether an external proxy forwards the traffic and the client only decides.
func (c *Config) Validator() bool { return c.Mode == ModeValidator }

// NamespaceIRI is the expected namespace IRI {trust.hub}/v1/namespaces/{trust.namespace}.
func (c *Config) NamespaceIRI() string {
	return strings.TrimSuffix(c.Trust.Hub, "/") + "/v1/namespaces/" + c.Trust.Namespace
}
