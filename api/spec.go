// Package api provides the canonical OpenAPI specification for Alexandria.
// It exposes the embedded specification document for driving documentation adapters.
package api

import (
	_ "embed"
)

// Spec contains the canonical embedded OpenAPI 3.0 specification in YAML format.
//
//go:embed openapi.yaml
var Spec []byte
