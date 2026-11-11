// Tests server.json loading: the deploy copy of the schema matches the embedded one, and the demo configuration loads with defaults.
// Fixtures: deploy/server.json and deploy/server.schema.json (DP-11).
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
	if c.Context() != c.Base+"/pub/_vhub/v1/context.jsonld" || c.Auth.AdminClaim.Value != "vhub-admin" || c.Compiler != "poc" {
		t.Fatalf("got %+v", c)
	}
}

func TestUnknownFieldRejected(t *testing.T) {
	p := t.TempDir() + "/server.json"
	_ = os.WriteFile(p, []byte(`{"listen":"a:1","base":"http://h","signing":{"key":"file:k"},"auth":{"issuers":[{"iss":"i","discovery_url":"http://i"}],"audience":"a"},"storage":{"database":"file:d","blobs":"file:b"},"x":1}`), 0o600)
	if _, err := Load(p); err == nil {
		t.Fatal("unknown field accepted")
	}
}
