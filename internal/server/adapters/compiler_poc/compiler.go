// Minimal profile compiler for the PoC draft format of Part 5.1: N1–N5 checks and package compilation.
// Implements the ProfileCompiler port so the real DSL compiler can replace it without touching the domain (DP-6, REQ-HUB-001).
package compiler_poc

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Allowed vocabularies of the PoC format; slot names follow the A.4 finding enum (agent decision).
var (
	slotNames   = map[string]bool{"odrl": true, "dcat": true, "dsp": true, "dcp": true}
	obligations = map[string]bool{"mandatory": true, "recommended": true, "optional": true}
	roles       = map[string]bool{"json-schema": true, "shacl": true, "context": true}
	gates       = map[string]bool{"dsp_message": true, "policy_intake": true, "catalog_entry": true}
)

// slotDecl is one entry of the draft slots[].
type slotDecl struct {
	Slot       string   `json:"slot"`
	Obligation string   `json:"obligation"`
	Profiles   []string `json:"profiles,omitempty"`
}

// fileDecl is one entry of the draft files[].
type fileDecl struct {
	Path      string          `json:"path"`
	MediaType string          `json:"media_type"`
	Role      string          `json:"role"`
	Slot      string          `json:"slot"`
	AppliesTo []string        `json:"applies_to"`
	URL       string          `json:"url,omitempty"`
	Content   json.RawMessage `json:"content"`
}

// draft is the Part 5.1 document.
type draft struct {
	Space     string          `json:"space"`
	Version   string          `json:"version"`
	Lifecycle json.RawMessage `json:"lifecycle,omitempty"`
	Slots     []slotDecl      `json:"slots"`
	Files     []fileDecl      `json:"files"`
}

// Compiler is the compiler_poc adapter; Defaults fill an absent or partial lifecycle.
type Compiler struct{ Defaults lifecycle.Params }

var _ app.ProfileCompiler = Compiler{}

// finding builds a coded error at a JSON pointer.
func finding(c codes.Code, ptr, format string, args ...any) *codes.Error {
	return &codes.Error{Code: c, Path: ptr, Detail: fmt.Sprintf(format, args...)}
}

// Validate runs N1–N3, or N1–N5 when level is 5, returning one finding per problem.
func (c Compiler) Validate(doc []byte, level int, ctx app.CompileContext) []*codes.Error {
	d, _, out := c.parse(doc, ctx)
	if d == nil || len(out) > 0 {
		return out
	}
	for i, f := range d.Files {
		if f.Role != "json-schema" {
			continue
		}
		if _, err := schemas.Compile(f.Content); err != nil {
			out = append(out, finding(PocN3, fmt.Sprintf("/files/%d/content", i), "%s does not compile: %v", f.Path, err))
		}
	}
	if level < 5 || len(out) > 0 {
		return out
	}
	v, _ := semver.Parse(d.Version)
	if _, ok := lifecycle.ClassOf(v, ctx.Latest); !ok {
		out = append(out, finding(PocN4, "/version", "version %s is not greater than the latest published %s", v, ctx.Latest))
	}
	return append(out, n5(d)...)
}

// parse runs N1 and N2 and returns the decoded draft and its lifecycle.
func (c Compiler) parse(doc []byte, ctx app.CompileContext) (*draft, lifecycle.Params, []*codes.Error) {
	trimmed := bytes.TrimSpace(doc)
	if !json.Valid(doc) || len(trimmed) == 0 || trimmed[0] != '{' {
		return nil, lifecycle.Params{}, []*codes.Error{finding(PocN1, "", "draft is not a well-formed JSON object")}
	}
	dec := json.NewDecoder(bytes.NewReader(doc))
	dec.DisallowUnknownFields()
	var d draft
	if err := dec.Decode(&d); err != nil {
		return nil, lifecycle.Params{}, []*codes.Error{finding(PocN2, "", "draft structure: %v", err)}
	}
	var out []*codes.Error
	if d.Space == "" {
		out = append(out, finding(PocN2, "/space", "space is required"))
	} else if ctx.Space != "" && d.Space != ctx.Space {
		out = append(out, finding(PocN2, "/space", "this hub serves %s, not %s", ctx.Space, d.Space))
	}
	if _, err := semver.Parse(d.Version); err != nil {
		out = append(out, finding(PocN2, "/version", "%v", err))
	}
	if len(d.Slots) == 0 {
		out = append(out, finding(PocN2, "/slots", "at least one slot is required"))
	}
	for i, s := range d.Slots {
		if !slotNames[s.Slot] || !obligations[s.Obligation] {
			out = append(out, finding(PocN2, fmt.Sprintf("/slots/%d", i), "slot %q with obligation %q is not allowed", s.Slot, s.Obligation))
		}
	}
	out = append(out, checkFiles(d.Files)...)
	params, err := lifecycle.ParseOver(c.Defaults, d.Lifecycle)
	if err != nil {
		return &d, params, append(out, finding(PocN2, "/lifecycle", "%v", err))
	}
	for g, m := range params.Degraded {
		if (m != "continue" && m != "reject") || !(gates[g] || g == "agreement_admission") {
			out = append(out, finding(PocN2, "/lifecycle/degraded/"+g, "degraded mode %q is not allowed", m))
		}
	}
	return &d, params, append(out, params.Validate()...)
}

// checkFiles validates the structure of each file entry, including D-PVP-1.
func checkFiles(files []fileDecl) []*codes.Error {
	var out []*codes.Error
	seen := map[string]bool{}
	for i, f := range files {
		ptr := fmt.Sprintf("/files/%d", i)
		clean := path.Clean(f.Path)
		switch {
		case f.Path == "" || clean != f.Path || strings.HasPrefix(f.Path, "/") || strings.HasPrefix(clean, ".."):
			out = append(out, finding(PocN2, ptr+"/path", "path %q is not a clean relative path", f.Path))
		case f.Path == "manifest.json" || f.Path == "space-lock.json" || seen[f.Path]:
			out = append(out, finding(PocN2, ptr+"/path", "path %q is reserved or repeated", f.Path))
		}
		seen[f.Path] = true
		if f.MediaType == "" || !roles[f.Role] || len(f.Content) == 0 {
			out = append(out, finding(PocN2, ptr, "media_type, role and content are required; role %q", f.Role))
		}
		if f.MediaType == "application/ld+json" && f.URL == "" {
			out = append(out, finding(PocN2, ptr+"/url", "application/ld+json files require url (D-PVP-1)"))
		}
		for _, g := range f.AppliesTo {
			if !gates[g] {
				out = append(out, finding(PocN2, ptr+"/applies_to", "gate %q is not allowed", g))
			}
		}
	}
	return out
}

// n5 checks internal coherence of slots and files.
func n5(d *draft) []*codes.Error {
	var out []*codes.Error
	declared := map[string]bool{}
	for i, s := range d.Slots {
		if declared[s.Slot] {
			out = append(out, finding(PocN5, fmt.Sprintf("/slots/%d", i), "slot %s is declared twice", s.Slot))
		}
		declared[s.Slot] = true
	}
	used := map[string]bool{}
	for i, f := range d.Files {
		used[f.Slot] = true
		if !declared[f.Slot] {
			out = append(out, finding(PocN5, fmt.Sprintf("/files/%d/slot", i), "file %s binds undeclared slot %q", f.Path, f.Slot))
		}
	}
	for i, s := range d.Slots {
		if s.Obligation != "optional" && !used[s.Slot] {
			out = append(out, finding(PocN5, fmt.Sprintf("/slots/%d", i), "slot %s has no artefact", s.Slot))
		}
	}
	return out
}

// Compile produces manifest.json, space-lock.json and the files, all in canonical form.
func (c Compiler) Compile(doc []byte, ctx app.CompileContext) (domain.Compiled, error) {
	d, params, errs := c.parse(doc, ctx)
	if d == nil || len(errs) > 0 {
		return domain.Compiled{}, errors.New("compiler_poc: draft does not pass N1–N2")
	}
	v, _ := semver.Parse(d.Version)
	class, ok := lifecycle.ClassOf(v, ctx.Latest)
	if !ok {
		return domain.Compiled{}, errors.New("compiler_poc: version does not increase")
	}
	files := map[string][]byte{}
	var entries []pkgv.File
	for _, f := range d.Files {
		if f.MediaType == "application/ld+json" && f.URL == "" {
			return domain.Compiled{}, fmt.Errorf("compiler_poc: %s lacks url (D-PVP-1)", f.Path)
		}
		b, err := fileBytes(f.Content)
		if err != nil {
			return domain.Compiled{}, err
		}
		files[f.Path] = b
		at := append([]string{}, f.AppliesTo...)
		sort.Strings(at)
		entries = append(entries, pkgv.File{Path: f.Path, MediaType: f.MediaType, Digest: ni.Sum(b), Role: f.Role, Slot: f.Slot, AppliesTo: at, URL: f.URL})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var slots []pkgv.Slot
	profiles := map[string][]string{}
	for _, s := range d.Slots {
		slots = append(slots, pkgv.Slot{Slot: s.Slot, Obligation: s.Obligation})
		if len(s.Profiles) > 0 {
			profiles[s.Slot] = s.Profiles
		}
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	if entries == nil {
		entries = []pkgv.File{}
	}
	manifest, err := canon.Marshal(pkgv.Manifest{Space: d.Space, Version: v.String(), Files: entries, Slots: slots})
	if err != nil {
		return domain.Compiled{}, err
	}
	if len(profiles) == 0 {
		profiles = nil
	}
	lock, err := canon.Marshal(pkgv.SpaceLock{Space: d.Space, Version: v.String(), Manifest: ni.Sum(manifest), Lifecycle: params, Profiles: profiles})
	if err != nil {
		return domain.Compiled{}, err
	}
	return domain.Compiled{Space: d.Space, Version: v, Class: class, Params: params, Manifest: manifest, SpaceLock: lock, Files: files, Entries: entries}, nil
}

// fileBytes stores a JSON string as its text and any other JSON value in canonical form.
func fileBytes(content json.RawMessage) ([]byte, error) {
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return []byte(s), nil
	}
	return canon.Transform(content)
}
