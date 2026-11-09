// Vendored profiles: external specifications copied and pinned locally, such as DCAT-AP shapes or DSP schemas.
// Lookup reads <slot>/<version>/vendor.json from a file system and never touches the network (§11.3 source: vendored).
package dsl

import (
	"encoding/json"
	"io/fs"
	"path"
)

// VendoredArtifact is one file of a vendored profile.
type VendoredArtifact struct {
	Path      string `json:"path"`
	Role      string `json:"role"`
	MediaType string `json:"media_type"`
	URL       string `json:"url,omitempty"`
	Bytes     []byte `json:"-"`
}

// Vendored is a pinned copy of an external profile.
type Vendored struct {
	Profile     string             `json:"profile"`
	Version     string             `json:"version"`
	IsProfileOf []string           `json:"is_profile_of,omitempty"`
	Artifacts   []VendoredArtifact `json:"artifacts"`
}

// Vendor resolves vendored bindings.
type Vendor interface {
	Lookup(slot, profile, version string) (*Vendored, bool)
}

// FSVendor reads vendored profiles from <slot>/<version>/vendor.json in a file system.
type FSVendor struct{ FS fs.FS }

// Lookup loads the vendored copy of a profile version, or reports false.
func (v FSVendor) Lookup(slot, profile, version string) (*Vendored, bool) {
	if v.FS == nil {
		return nil, false
	}
	dir := path.Join(slot, version)
	b, err := fs.ReadFile(v.FS, path.Join(dir, "vendor.json"))
	if err != nil {
		return nil, false
	}
	var out Vendored
	if json.Unmarshal(b, &out) != nil || out.Profile != profile {
		return nil, false
	}
	for i := range out.Artifacts {
		if out.Artifacts[i].Bytes, err = fs.ReadFile(v.FS, path.Join(dir, out.Artifacts[i].Path)); err != nil {
			return nil, false
		}
	}
	return &out, true
}
