// Tests server configuration loading in JSON and YAML for both Appendix B and the config.yaml section layout.
// Fixtures: deploy/server.json, deploy/server.yaml, config/server.yaml and deploy/server.schema.json (DP-11).
package config

import (
	"bytes"
	"os"
	"testing"
)

func TestDeploySchemaMatchesEmbedded(t *testing.T) {
	b, err := os.ReadFile("../../../deploy/server.schema.json")
	if err != nil || !bytes.Equal(b, Schema()) {
		t.Fatal("deploy/server.schema.json differs from the embedded schema")
	}
}

func TestDemoConfigLoads(t *testing.T) {
	c, err := Load("../../../deploy/server.json")
	if err != nil {
		t.Fatal(err)
	}
	if c.Context() != c.Base+"/pub/_vhub/v1/context.jsonld" || c.Auth.AdminClaim.Value != "vhub-admin" || c.Compiler != "dsl" {
		t.Fatalf("got %+v", c)
	}
}

// TestExplicitPoCCompilerPreserved verifies that setting compiler to poc overrides the dsl default.
func TestExplicitPoCCompilerPreserved(t *testing.T) {
	raw := []byte(`{"listen":"127.0.0.1:8443","base":"https://127.0.0.1:8443","signing":{"key":"file:k"},"auth":{"issuers":[{"iss":"i","discovery_url":"http://i"}],"audience":"a"},"storage":{"database":"file:d","blobs":"file:b"},"compiler":"poc"}`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Compiler != "poc" {
		t.Fatalf("got compiler %q, want poc", c.Compiler)
	}
}

func TestYAMLAndNodeLayoutMatchJSON(t *testing.T) {
	baseCfg, err := Load("../../../deploy/server.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"../../../deploy/server.yaml", "../../../config/server.yaml"} {
		c, err := Load(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if c.Listen != baseCfg.Listen || c.Base != baseCfg.Base || c.Context() != baseCfg.Context() ||
			c.Compiler != baseCfg.Compiler || c.VendorDir != baseCfg.VendorDir ||
			c.Auth.Audience != baseCfg.Auth.Audience || len(c.Auth.Issuers) != 1 ||
			c.Auth.Issuers[0] != baseCfg.Auth.Issuers[0] || c.Signing.Key != baseCfg.Signing.Key ||
			c.Storage.Database != baseCfg.Storage.Database || c.Storage.Blobs != baseCfg.Storage.Blobs {
			t.Fatalf("%s mismatch:\ngot  %+v\nwant %+v", path, c, baseCfg)
		}
	}
}

func TestNodeLayoutJSONAndUnknownFieldRejected(t *testing.T) {
	raw := []byte(`{
		"common_config": {
			"hosts": {"http": {"protocol": "https", "url": "127.0.0.1", "port": "8443", "internal_port": "1234"}},
			"db": {"db_type": "SQLite", "database": "file:d", "blobs": "file:b"},
			"connection": {"is_local": true, "has_tls_proxy": true}
		},
		"auth_config": {"issuer": "http://127.0.0.1:9400", "audience": "alexandria-vhub"},
		"did_config": {"type": "web", "domain": "hub.example.com", "signing_key": "file:k"}
	}`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.Listen != "127.0.0.1:1234" || c.Base != "https://127.0.0.1:8443" || c.DID != "did:web:hub.example.com" {
		t.Fatalf("got %+v", c)
	}
	if _, err := Parse([]byte(`{"common_config":{"unknown":1}}`)); err == nil {
		t.Fatal("unknown field inside common_config accepted")
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	p := t.TempDir() + "/server.json"
	_ = os.WriteFile(p, []byte(`{"listen":"a:1","base":"http://h","signing":{"key":"file:k"},"auth":{"issuers":[{"iss":"i","discovery_url":"http://i"}],"audience":"a"},"storage":{"database":"file:d","blobs":"file:b"},"x":1}`), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("unknown field accepted")
	}
}

// TestObservabilityNodeLayoutParsed verifies observability fields in the config.yaml section layout.
func TestObservabilityNodeLayoutParsed(t *testing.T) {
	raw := []byte(`{
		"common_config": {
			"hosts": {"http": {"protocol": "https", "url": "127.0.0.1", "port": "8443"}},
			"db": {"database": "file:d", "blobs": "file:b"},
			"connection": {"is_prod": true}
		},
		"auth_config": {"issuer": "https://idp.example.com", "audience": "alexandria-vhub"},
		"did_config": {"signing_key": "file:k"},
		"observability": {
			"log_level": "debug",
			"log_format": "json",
			"otlp_endpoint": "http://127.0.0.1:4318",
			"otlp_insecure": true,
			"service_name": "vhub-prod"
		}
	}`)
	c, err := Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	obs := c.Observability
	if obs.LogLevel != "debug" || obs.LogFormat != "json" || obs.OTLPEndpoint != "http://127.0.0.1:4318" || !obs.OTLPInsecure || obs.ServiceName != "vhub-prod" || obs.Environment != "production" {
		t.Fatalf("got %+v", obs)
	}
}

// TestInvalidObservabilitySettingsRejected verifies rejection of unsupported log levels and formats.
func TestInvalidObservabilitySettingsRejected(t *testing.T) {
	base := `{"listen":"a:1","base":"http://h","signing":{"key":"file:k"},"auth":{"issuers":[{"iss":"i","discovery_url":"http://i"}],"audience":"a"},"storage":{"database":"file:d","blobs":"file:b"},`
	if _, err := Parse([]byte(base + `"observability":{"log_level":"verbose"}}`)); err == nil {
		t.Fatal("expected invalid log_level to fail")
	}
	if _, err := Parse([]byte(base + `"observability":{"log_format":"xml"}}`)); err == nil {
		t.Fatal("expected invalid log_format to fail")
	}
}

// TestPostgresDatabaseConfig verifies PostgreSQL DSN acceptance and db_type validation in both layouts.
func TestPostgresDatabaseConfig(t *testing.T) {
	dsn := "postgres://alexandria:secret@postgres:5432/alexandria_hub?sslmode=disable"
	flat := `{"listen":"0.0.0.0:8080","base":"https://hub.example.com","signing":{"key":"file:/var/hub.key"},"auth":{"issuers":[{"iss":"https://idp","discovery_url":"https://idp/.well-known/openid-configuration"}],"audience":"a"},"storage":{"database":"` + dsn + `","blobs":"file:/var/blobs"}}`
	c, err := Parse([]byte(flat))
	if err != nil || c.Storage.Database != dsn {
		t.Fatalf("flat postgres config: err=%v db=%q", err, c.Storage.Database)
	}
	node := `{"common_config":{"hosts":{"http":{"protocol":"https","url":"hub.example.com","internal_port":"8080"}},"db":{"db_type":"Postgres","database":"` + dsn + `","blobs":"file:/var/blobs"},"connection":{"has_tls_proxy":true}},"auth_config":{"issuer":"https://idp","audience":"a"},"did_config":{"signing_key":"file:/var/hub.key"}}`
	p := t.TempDir() + "/server.json"
	_ = os.WriteFile(p, []byte(node), 0o600)
	loaded, err := Load(p)
	if err != nil || loaded.Storage.Database != dsn {
		t.Fatalf("node postgres config: err=%v db=%q", err, loaded.Storage.Database)
	}
	badMismatch := `{"common_config":{"hosts":{"http":{"protocol":"https","url":"h","port":"8443"}},"db":{"db_type":"Postgres","database":"file:hub.db","blobs":"file:b"},"connection":{}},"auth_config":{"issuer":"https://idp","audience":"a"},"did_config":{"signing_key":"file:k"}}`
	if _, err := Parse([]byte(badMismatch)); err == nil {
		t.Fatal("expected db_type Postgres with file: database to fail")
	}
}
