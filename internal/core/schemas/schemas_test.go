// Tests that the Appendix A schemas compile and that the Annex C examples validate against them.
// Also checks the embedded copies are byte-identical to spec/schemas (checks H1, C2; REQ-DOC-001…REQ-DOC-006).
package schemas

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// specDir is the location of the verbatim Annex C schemas and examples.
var specDir = filepath.Join("..", "..", "..", "spec")

// examples maps each schema to its Annex C example.
var examples = map[Kind]string{
	Versions: "versions.json", Version: "version.json", Config: "config.json",
	Verdict: "verdict-event.json", Manifest: "manifest.json", Rules: "rules.json",
}

// readSpec loads a file under spec/.
func readSpec(t *testing.T, parts ...string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{specDir}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestExamplesValidate(t *testing.T) {
	for k, ex := range examples {
		if err := Validate(k, readSpec(t, "examples", ex)); err != nil {
			t.Errorf("%s: %v", ex, err)
		}
	}
}

func TestEmbeddedCopiesMatchSpec(t *testing.T) {
	for _, k := range All {
		if !bytes.Equal(readSpec(t, "schemas", string(k)), Raw(k)) {
			t.Errorf("%s differs from spec/schemas", k)
		}
	}
}

func TestContextMatchesSpec(t *testing.T) {
	if !bytes.Equal(readSpec(t, "schemas", "context.jsonld"), Context()) {
		t.Error("context.jsonld differs from spec/schemas")
	}
}

func TestDidTTLLookaheadEmulated(t *testing.T) {
	base := `{"trust":{"hub_did":"did:web:h","hub":"https://h","namespace":"demo","did_ttl":"%s"},"upstreams":{"dsp":"http://c"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"}}`
	for v, ok := range map[string]bool{"PT6H": true, "P1D": true, "P": false, "PT": false} {
		err := Validate(Config, []byte(fmtS(base, v)))
		if (err == nil) != ok {
			t.Errorf("did_ttl %s: err=%v", v, err)
		}
	}
}

// fmtS substitutes the single %s of a template.
func fmtS(tpl, v string) string {
	return string(bytes.Replace([]byte(tpl), []byte("%s"), []byte(v), 1))
}
