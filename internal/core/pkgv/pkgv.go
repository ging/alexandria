// Package pkgv defines the published package model (manifest.json, space-lock.json, files) and its offline verification S8, S11–S13.
// The hub produces packages with this model and the client verifies them without network (REQ-CLI-006, REQ-CLI-007, DP-6).
package pkgv

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
)

// File is one entry of manifest.files[]; URL is mandatory for application/ld+json (D-PVP-1).
type File struct {
	Path      string    `json:"path"`
	MediaType string    `json:"media_type"`
	Digest    ni.Digest `json:"digest"`
	Role      string    `json:"role"`
	Slot      string    `json:"slot"`
	AppliesTo []string  `json:"applies_to"`
	URL       string    `json:"url,omitempty"`
}

// Slot is one binding obligation of the package.
type Slot struct {
	Slot       string `json:"slot"`
	Obligation string `json:"obligation"`
}

// Manifest is manifest.json (Part 5.2).
type Manifest struct {
	Space   string `json:"space"`
	Version string `json:"version"`
	Files   []File `json:"files"`
	Slots   []Slot `json:"slots"`
}

// SpaceLock is space-lock.json; Lifecycle and Profiles are agent decisions needed by S7 and §4.9.
type SpaceLock struct {
	Space     string              `json:"space"`
	Version   string              `json:"version"`
	Manifest  ni.Digest           `json:"manifest"`
	Lifecycle lifecycle.Params    `json:"lifecycle"`
	Profiles  map[string][]string `json:"profiles,omitempty"`
}

// Files is the raw material of a package as fetched or stored.
type Files struct {
	Manifest  []byte
	Sig       []byte
	Receipt   []byte
	SpaceLock []byte
	Content   map[string][]byte
}

// Package is a verified package ready to be used by classification.
type Package struct {
	Digest    ni.Digest
	Manifest  Manifest
	SpaceLock SpaceLock
	Raw       Files
}

// Verify applies S8, S11, S12 and S13 for the listing entry (version, expected digest).
func Verify(f Files, expected ni.Digest, version string) (*Package, *codes.Error) {
	if ni.Sum(f.Manifest) != expected {
		return nil, codes.New(codes.SyncManifestDigestMismatch, "manifest.json of %s does not match %s", version, expected)
	}
	var m Manifest
	if err := json.Unmarshal(f.Manifest, &m); err != nil {
		return nil, codes.New(codes.SyncIncoherent, "manifest.json of %s: %v", version, err)
	}
	return check(f, m, expected, version)
}

// VerifyFiles applies S11 and S12 to an already parsed manifest.
func VerifyFiles(f Files, m Manifest) *codes.Error {
	listed := map[string]bool{}
	for _, e := range m.Files {
		listed[e.Path] = true
		if b, ok := f.Content[e.Path]; ok && ni.Sum(b) != e.Digest {
			return codes.New(codes.SyncFileHashMismatch, "%s does not match its digest", e.Path)
		}
	}
	for p := range f.Content {
		if !listed[p] {
			return codes.New(codes.SyncFileSetMismatch, "%s is not listed in files[]", p)
		}
	}
	for p := range listed {
		if _, ok := f.Content[p]; !ok {
			return codes.New(codes.SyncFileSetMismatch, "%s is missing", p)
		}
	}
	return nil
}

// check runs S11–S13 once the manifest digest is known to be right.
func check(f Files, m Manifest, digest ni.Digest, version string) (*Package, *codes.Error) {
	if err := VerifyFiles(f, m); err != nil {
		return nil, err
	}
	var sl SpaceLock
	if err := json.Unmarshal(f.SpaceLock, &sl); err != nil {
		return nil, codes.New(codes.SyncIncoherent, "space-lock.json of %s: %v", version, err)
	}
	if m.Version != version || sl.Manifest != digest || sl.Version != m.Version || sl.Space != m.Space {
		return nil, codes.New(codes.SyncIncoherent, "manifest %s / space-lock %s do not match listing entry %s", m.Version, sl.Version, version)
	}
	return &Package{Digest: digest, Manifest: m, SpaceLock: sl, Raw: f}, nil
}

// Paths returns the file paths of the manifest in order.
func (m Manifest) Paths() []string {
	out := make([]string, 0, len(m.Files))
	for _, f := range m.Files {
		out = append(out, f.Path)
	}
	sort.Strings(out)
	return out
}

// Obligation returns the obligation of a slot, or an empty string if unbound.
func (m Manifest) Obligation(slot string) string {
	for _, s := range m.Slots {
		if s.Slot == slot {
			return s.Obligation
		}
	}
	return ""
}

// URLs returns the set of context URLs declared by the manifest (D-PVP-1).
func (m Manifest) URLs() map[string]bool {
	out := map[string]bool{}
	for _, f := range m.Files {
		if f.URL != "" {
			out[f.URL] = true
		}
	}
	return out
}

// String describes the package for logs.
func (p *Package) String() string { return fmt.Sprintf("%s@%s", p.Manifest.Version, p.Digest) }
