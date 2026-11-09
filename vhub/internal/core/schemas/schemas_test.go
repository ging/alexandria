// Tests that the Appendix A schemas compile and that the spec examples validate against them.
// Also checks the embedded copies are byte-identical to conformance/schemas (check H1, C2).
package schemas

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// conformanceDir is the location of the verbatim schemas and examples.
var conformanceDir = filepath.Join("..", "..", "..", "conformance")

func TestExamplesValidate(t *testing.T) {
	for k, ex := range map[Kind]string{Versions: "versions.json", Version: "version.json", Config: "config.json", Verdict: "verdict-event.json"} {
		b, err := os.ReadFile(filepath.Join(conformanceDir, "examples", ex))
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate(k, b); err != nil {
			t.Errorf("%s: %v", ex, err)
		}
	}
}

func TestEmbeddedCopiesMatchConformance(t *testing.T) {
	for _, k := range []Kind{Versions, Version, Config, Verdict} {
		b, err := os.ReadFile(filepath.Join(conformanceDir, "schemas", string(k)))
		if err != nil || !bytes.Equal(b, Raw(k)) {
			t.Errorf("%s differs from conformance/schemas", k)
		}
	}
}

func TestDidTTLLookaheadEmulated(t *testing.T) {
	base := `{"space":"x","trust":{"hub_did":"did:web:h","hub":"https://h","credentials":"file:///c","did_ttl":"%s"},"upstreams":{"dsp":"http://c"},"listeners":{"admin":{"bind":"a:1"}},"events":{"sink":"file:///e"}}`
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
