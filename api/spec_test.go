// Package api_test validates the embedded OpenAPI specification document.
// It verifies that the embedded contract is non-empty and decodes as valid YAML.
package api_test

import (
	"testing"

	"github.com/ging/alexandria/api"
	"gopkg.in/yaml.v3"
)

func TestEmbeddedSpecIsValidYAML(t *testing.T) {
	t.Parallel()

	if len(api.Spec) == 0 {
		t.Fatal("api.Spec is empty, want embedded OpenAPI specification")
	}

	var parsed any
	if err := yaml.Unmarshal(api.Spec, &parsed); err != nil {
		t.Fatalf("yaml.Unmarshal(api.Spec) = %v, want valid YAML", err)
	}
}
