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
		if !reflect.DeepEqual(c, baseCfg) {
			t.Fatalf("%s mismatch:\ngot  %+v\nwant %+v", path, c, baseCfg)
		}
	}
	if _, cerr := Parse([]byte(`{"common_config":{"unknown":true}}`)); cerr == nil || cerr.Code != codes.ConfigSchema {
		t.Fatalf("expected CONFIG_SCHEMA on unknown field in common_config, got %v", cerr)
	}
}
