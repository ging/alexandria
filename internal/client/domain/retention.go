// Package domain decides which packages the periodic collection keeps: listed versions and versions of registered agreements.
// Everything else may be deleted and re-downloaded if it reappears (§4.10, REQ-SYS-006, REQ-CLI-017, REQ-CLI-033).
package domain

import (
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
)

// Keep returns the manifests to keep: those in versions[] of the last listing and those referenced by agreements.
func Keep(listed []lifecycle.Record, agreements []ni.Digest) map[ni.Digest]bool {
	out := map[ni.Digest]bool{}
	for _, r := range listed {
		out[r.Manifest] = true
	}
	for _, d := range agreements {
		out[d] = true
	}
	return out
}
