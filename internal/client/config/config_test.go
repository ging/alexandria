// Tests CFG-1 on the Annex C example and a negative document, YAML/JSON loading for both Appendix A.3 and config.yaml layouts, and the §4.3.1 defaults (T-CLI-001, T-DOC-003).
// Fixtures: spec/examples/config.json, deploy/client.json, deploy/client.yaml, config/client.yaml and inline documents.
package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
)

// example reads the Annex C configuration.
func example(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile("../../../spec/examples/config.json")
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestT_DOC_003_ExampleLoadsWithDefaults(t *testing.T) {
	c, err := Parse(example(t))
	if err != nil {
		t.Fatal(err)
	}
	if c.Trust.Context != DefaultContext || c.NamespaceIRI() != "https://hub.example.com/v1/namespaces/demo" || c.Audit.Window != 100 || c.GCInterval != 86400 {
		t.Fatalf("got %+v", c)
	}
}

func TestT_CLI_001_UnknownRootFieldIsConfigSchema(t *testing.T) {
	_, err := Parse([]byte(`{"trust":{"hub":"https://h","namespace":"demo","hub_did":"did:web:h"},"upstreams":{"dsp":"http://c"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"},"routes":[]}`))
	if err == nil || err.Code != codes.ConfigSchema {
		t.Fatalf("got %v", err)
	}
}

func TestYAMLConfig(t *testing.T) {
	p := filepath.Join(t.TempDir(), "client.yaml")
	_ = os.WriteFile(p, []byte("trust: {hub: 'https://h', namespace: demo, hub_did: 'did:web:h'}\nupstreams: {dsp: 'http://c'}\nlisteners: {admin: {bind: 'a:1'}}\nevents: {sink: 'file:///e'}\n"), 0o600)
	if _, err := Load(p); err != nil {
		t.Fatal(err)
	}
}

func TestNodeLayoutMatchesDeployJSON(t *testing.T) {
	baseCfg, err := Load("../../../deploy/client.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../../deploy/client.yaml", "../../../config/client.yaml"} {
		c, err := Load(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if c.HubCA == "" {
			t.Fatalf("%s: expected HubCA to be resolved", path)
		}
		c.HubCA = ""
		if !reflect.DeepEqual(c, baseCfg) {
			t.Fatalf("%s mismatch:\ngot  %+v\nwant %+v", path, c, baseCfg)
		}
	}
	if _, cerr := Parse([]byte(`{"common_config":{"unknown":true}}`)); cerr == nil || cerr.Code != codes.ConfigSchema {
		t.Fatalf("expected CONFIG_SCHEMA on unknown field in common_config, got %v", cerr)
	}
}

// TestObservabilityNodeLayoutParsed verifies observability fields in the config.yaml section layout.
func TestObservabilityNodeLayoutParsed(t *testing.T) {
	raw := []byte(`{
		"common_config": {
			"hosts": {"http": {"protocol": "http", "url": "127.0.0.1", "port": "9080"}},
			"connection": {"is_prod": true}
		},
		"did_config": {"hub_did": "did:key:z6MkpTHR8VNsBxYAAWHut2Geadd9jSwuBV8xRoAnwWsdvktH"},
		"observability": {
			"log_level": "warn",
			"log_format": "json",
			"otlp_endpoint": "http://127.0.0.1:4318",
			"otlp_insecure": true,
			"service_name": "client-prod",
			"port": "9090",
			"events_sink": "file:var/events.jsonl"
		},
		"client_config": {
			"trust": {"hub": "https://hub.example.com", "namespace": "demo"},
			"upstreams": {"dsp": "http://127.0.0.1:8282/dsp"}
		}
	}`)
	c, cerr := Parse(raw)
	if cerr != nil {
		t.Fatal(cerr)
	}
	if c.Log.Level != "warn" || c.Log.Format != "json" || c.Log.OTLPEndpoint != "http://127.0.0.1:4318" || !c.Log.OTLPInsecure || c.Log.ServiceName != "client-prod" || c.Log.Environment != "production" {
		t.Fatalf("got %+v", c.Log)
	}
}

// TestInvalidObservabilitySettingsRejected verifies CONFIG_SCHEMA rejection on invalid log levels or formats.
func TestInvalidObservabilitySettingsRejected(t *testing.T) {
	badLevel := []byte("common_config: {}\ndid_config: {hub_did: 'did:key:z6Mk'}\nobservability: {log_level: bogus, port: '9090', events_sink: 'file:///e'}\nclient_config: {trust: {hub: 'https://h', namespace: demo}, upstreams: {dsp: 'http://c'}}\n")
	if _, cerr := Parse(badLevel); cerr == nil || cerr.Code != codes.ConfigSchema {
		t.Fatalf("expected CONFIG_SCHEMA on invalid log_level, got %v", cerr)
	}
	badFormat := []byte("common_config: {}\ndid_config: {hub_did: 'did:key:z6Mk'}\nobservability: {log_format: xml, port: '9090', events_sink: 'file:///e'}\nclient_config: {trust: {hub: 'https://h', namespace: demo}, upstreams: {dsp: 'http://c'}}\n")
	if _, cerr := Parse(badFormat); cerr == nil || cerr.Code != codes.ConfigSchema {
		t.Fatalf("expected CONFIG_SCHEMA on invalid log_format, got %v", cerr)
	}
}

func TestModeDefaultsToProxy(t *testing.T) {
	c, err := Parse(example(t))
	if err != nil || c.Mode != ModeProxy || c.Validator() {
		t.Fatalf("got %v %+v", err, c)
	}
}

func TestValidatorModeNeedsNoUpstreams(t *testing.T) {
	c, err := Parse([]byte(`{"mode":"validator","trust":{"hub":"https://h","namespace":"demo","hub_did":"did:web:h"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"}}`))
	if err != nil || !c.Validator() {
		t.Fatalf("got %v %+v", err, c)
	}
}

func TestProxyModeStillRequiresUpstreams(t *testing.T) {
	for _, mode := range []string{``, `"mode":"proxy",`} {
		_, err := Parse([]byte(`{` + mode + `"trust":{"hub":"https://h","namespace":"demo","hub_did":"did:web:h"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"}}`))
		if err == nil || err.Code != codes.ConfigSchema {
			t.Fatalf("mode %q: got %v", mode, err)
		}
	}
}

func TestUnknownModeIsConfigSchema(t *testing.T) {
	_, err := Parse([]byte(`{"mode":"sidecar","trust":{"hub":"https://h","namespace":"demo","hub_did":"did:web:h"},"upstreams":{"dsp":"http://c"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"}}`))
	if err == nil || err.Code != codes.ConfigSchema {
		t.Fatalf("got %v", err)
	}
}

func TestValidatorModeInNodeLayout(t *testing.T) {
	c, err := Parse([]byte("did_config: {hub_did: 'did:web:h'}\nobservability: {port: '9090', events_sink: 'file:///e'}\nclient_config: {mode: validator, trust: {hub: 'https://h', namespace: demo}, listeners: {dsp-ingress: {bind: '127.0.0.1:9080'}}}\n"))
	if err != nil || !c.Validator() || c.Upstreams.DSP != "" {
		t.Fatalf("got %v %+v", err, c)
	}
}

func TestExampleConfigsLoadInTheirMode(t *testing.T) {
	for path, validator := range map[string]bool{"../../../examples/proxy/client.yaml": false, "../../../examples/envoy/client.yaml": true} {
		c, err := Load(path)
		if err != nil || c.Validator() != validator {
			t.Fatalf("%s: %v %+v", path, err, c)
		}
	}
}
