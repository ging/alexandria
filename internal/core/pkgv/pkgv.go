// Package pkgv defines the package model (signed manifest credential plus files) and its offline verification S8, S11–S13.
// The hub produces packages with this model and the client verifies them without network (§3.7, REQ-CLI-006, REQ-CLI-007).
package pkgv

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/vc"
)

// Artifact roles of §3.7.
const (
	RoleJSONSchema = "json-schema"
	RoleSHACL      = "shacl"
	RoleContext    = "context"
	RoleOther      = "other"
)

// SlotsPath is the package file that carries the slot obligations of the SpaceProfile (agent decision, see DECISIONS.md).
const SlotsPath = "profile/slots.json"

// File is one entry of files[]; URL is mandatory for application/ld+json (D-PVP-1), Source cites a vocabulary revision (D-VOC-1).
type File struct {
	Path      string    `json:"path"`
	MediaType string    `json:"media_type"`
	Digest    ni.Digest `json:"digest"`
	Role      string    `json:"role"`
	Slot      string    `json:"slot,omitempty"`
	AppliesTo []string  `json:"applies_to,omitempty"`
	URL       string    `json:"url,omitempty"`
	Source    string    `json:"source,omitempty"`
}

// Manifest is the credentialSubject of a PackageManifest (§3.7).
type Manifest struct {
	ID        string           `json:"id"`
	Namespace string           `json:"namespace"`
	Version   string           `json:"version"`
	Lifecycle lifecycle.Params `json:"lifecycle"`
	Profiles  []string         `json:"profiles,omitempty"`
	Files     []File           `json:"files"`
}

// Slot is one binding of the SpaceProfile: its obligation and the gates it applies to.
type Slot struct {
	Slot       string   `json:"slot"`
	Obligation string   `json:"obligation"`
	AppliesTo  []string `json:"applies_to"`
}

// Files is the raw material of a package as fetched or stored.
type Files struct {
	Manifest []byte
	Content  map[string][]byte
}

// Package is a verified package ready to be used by classification.
type Package struct {
	Digest     ni.Digest
	Credential vc.Credential[Manifest]
	Manifest   Manifest
	Slots      []Slot
	Raw        Files
}

// Parse decodes the manifest credential of a package without checking it.
func Parse(raw []byte) (vc.Credential[Manifest], error) {
	var c vc.Credential[Manifest]
	if err := json.Unmarshal(raw, &c); err != nil {
		return c, fmt.Errorf("pkgv: %w", err)
	}
	return c, nil
}

// Verify applies S8, S11, S12 and S13 for the listing entry (namespace, version, expected digest).
func Verify(f Files, expected ni.Digest, namespace, version string) (*Package, *codes.Error) {
	if ni.Sum(f.Manifest) != expected {
		return nil, codes.New(codes.SyncManifestDigestMismatch, "manifest.json of %s does not match %s", version, expected)
	}
	c, err := Parse(f.Manifest)
	if err != nil {
		return nil, codes.New(codes.SyncMalformed, "manifest.json of %s: %v", version, err)
	}
	if e := VerifyFiles(f, c.Subject); e != nil {
		return nil, e
	}
	m := c.Subject
	if m.Version != version || m.Namespace != namespace {
		return nil, codes.New(codes.SyncIncoherent, "manifest %s of %s does not match listing entry %s of %s", m.Version, m.Namespace, version, namespace)
	}
	var slots []Slot
	if b, ok := f.Content[SlotsPath]; ok {
		_ = json.Unmarshal(b, &slots)
	}
	return &Package{Digest: expected, Credential: c, Manifest: m, Slots: slots, Raw: f}, nil
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
func (p *Package) Obligation(slot string) string {
	for _, s := range p.Slots {
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

// HasProfile reports whether iri is one of the compiled OdrlProfiles (§4.9).
func (m Manifest) HasProfile(iri string) bool {
	for _, p := range m.Profiles {
		if p == iri {
			return true
		}
	}
	return false
}

// String describes the package for logs.
func (p *Package) String() string { return fmt.Sprintf("%s@%s", p.Manifest.Version, p.Digest) }
