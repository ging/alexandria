// Tests of the client configuration: one negative document per invariant CFG-1..CFG-5 and the A.3 example.
// Realise T-CLI-001 and T-DOC-003 (§4.3, Appendix A.3).
package config

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
)

// base returns a valid configuration whose credentials file exists, with one placeholder for extra routes.
func base(t *testing.T, routes, extra string) []byte {
	t.Helper()
	cred := filepath.Join(t.TempDir(), "credentials.json")
	_ = os.WriteFile(cred, []byte(`{"token":"member-token"}`), 0o600)
	return []byte(`{"space":"https://example.com/spaces/demo","trust":{"hub_did":"did:web:hub.example.com","hub":"https://hub.example.com","credentials":"file://` + cred + `"},
"upstreams":{"dsp":"http://connector:8282/protocol"` + extra + `},"listeners":{"admin":{"bind":"127.0.0.1:9900"}},"routes":[` + routes + `],"events":{"sink":"file:///tmp/e.jsonl"}}`)
}

// credRe matches the credentials member of a configuration.
var credRe = regexp.MustCompile(`"credentials":"[^"]*"`)

// route renders one route of the given listener, path, message and gates.
func route(listener, path, msg, gates string) string {
	return `{"listener":"` + listener + `","match":{"method":"POST","path":"` + path + `"},"message":"` + msg + `","gates":[` + gates + `]}`
}

func TestT_CLI_001_ValidConfigLoads(t *testing.T) {
	c, err := Parse(base(t, route("dsp-ingress", "/negotiations/request", "ContractRequestMessage", `"dsp_message"`), ""))
	if err != nil || c.Token != "member-token" || c.DefaultAction != "forward" || c.Audit.Window != 100 {
		t.Fatalf("got %+v %v", c, err)
	}
}

func TestT_CLI_001_CFG1Schema(t *testing.T) {
	doc := strings.Replace(string(base(t, "", "")), `{"space"`, `{"unknown":1,"space"`, 1)
	if _, err := Parse([]byte(doc)); err == nil || err.Code != codes.ConfigSchema {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_001_CFG2Overlap(t *testing.T) {
	rs := route("dsp-ingress", "/negotiations/:a/request", "ContractRequestMessage", `"dsp_message"`) + "," + route("dsp-ingress", "/negotiations/:b/request", "ContractRequestMessage", `"dsp_message"`)
	if _, err := Parse(base(t, rs, "")); err == nil || err.Code != codes.ConfigRouteOverlap {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_001_CFG3GateMessage(t *testing.T) {
	if _, err := Parse(base(t, route("dsp-ingress", "/negotiations/request", "ContractRequestMessage", `"agreement_admission"`), "")); err == nil || err.Code != codes.ConfigGateMessageMismatch {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_001_CFG4Credentials(t *testing.T) {
	cfg := credRe.ReplaceAll(base(t, "", ""), []byte(`"credentials":"file:///no/existe"`))
	if _, err := Parse(cfg); err == nil || err.Code != codes.ConfigCredentialsUnreadable {
		t.Fatalf("got %v", err)
	}
}

func TestT_CLI_001_CFG5Upstream(t *testing.T) {
	if _, err := Parse(base(t, route("mgmt-ingress", "/v3/policydefinitions", "management", `"policy_intake"`), "")); err == nil || err.Code != codes.ConfigUpstreamMissing {
		t.Fatalf("got %v", err)
	}
}

func TestT_DOC_003_ExampleConfigValidates(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "conformance", "examples", "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, cerr := Parse(b); cerr == nil || cerr.Code != codes.ConfigCredentialsUnreadable {
		t.Fatalf("the example must pass CFG-1..3 and only fail CFG-4 on this machine: %v", cerr)
	}
}
