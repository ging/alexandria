// Package compiler_poc is the PoC profile compiler for the Part 5.1 draft format: N1–N5 checks and package compilation.
// Implements the ProfileCompiler port so the language compiler can replace it without touching the domain (DP-6, REQ-HUB-001).
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

// Allowed values of the PoC format.
var (
	obligations = map[string]bool{"mandatory": true, "recommended": true, "optional": true}
	roles       = map[string]bool{pkgv.RoleJSONSchema: true, pkgv.RoleSHACL: true, pkgv.RoleContext: true, pkgv.RoleOther: true}
	gates       = map[string]bool{"dsp_message": true, "policy_intake": true, "catalog_entry": true, "agreement_admission": true}
)

// slotDecl is one entry of the draft slots[].
type slotDecl struct {
	Slot       string   `json:"slot"`
	Obligation string   `json:"obligation"`
	AppliesTo  []string `json:"applies_to"`
}

// fileDecl is one entry of the draft files[]: exactly one of content or source.
type fileDecl struct {
	Path      string          `json:"path"`
	MediaType string          `json:"media_type"`
	Role      string          `json:"role"`
	Slot      string          `json:"slot,omitempty"`
	AppliesTo []string        `json:"applies_to,omitempty"`
	URL       string          `json:"url,omitempty"`
	Source    string          `json:"source,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`
}

// draft is the Part 5.1 document; the namespace comes from the route.
type draft struct {
	Version   string          `json:"version"`
	Lifecycle json.RawMessage `json:"lifecycle,omitempty"`
	Profiles  []string        `json:"profiles,omitempty"`
	Slots     []slotDecl      `json:"slots"`
	Files     []fileDecl      `json:"files"`
}

// Compiler is the compiler_poc adapter; Defaults fill an absent or partial lifecycle (§2.10).
type Compiler struct{ Defaults lifecycle.Params }

var _ app.ProfileCompiler = Compiler{}

// finding builds a coded error at a JSON pointer.
func finding(c codes.Code, ptr, format string, args ...any) *codes.Error {
	return &codes.Error{Code: c, Path: ptr, Detail: fmt.Sprintf(format, args...)}
}

// Validate runs N1–N3, or N1–N5 when level is 5, returning one finding per problem.
func (c Compiler) Validate(doc []byte, level int, ctx app.CompileContext) []*codes.Error {
	d, _, out := c.parse(doc)
	if d == nil || len(out) > 0 {
		return out
	}
	out = append(out, n3(d, ctx)...)
	if level < 5 || len(out) > 0 {
		return out
	}
	v, _ := semver.Parse(d.Version)
	if _, ok := lifecycle.ClassOf(v, ctx.Latest); !ok {
		out = append(out, finding(PocN4, "/version", "version %s is not greater than the latest published %s", v, ctx.Latest))
	}
	return append(out, n5(d, ctx)...)
}

// parse runs N1 and N2 and returns the decoded draft and its lifecycle with defaults applied.
func (c Compiler) parse(doc []byte) (*draft, lifecycle.Params, []*codes.Error) {
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
	if _, err := semver.Parse(d.Version); err != nil {
		out = append(out, finding(PocN2, "/version", "%v", err))
	}
	if len(d.Slots) == 0 {
		out = append(out, finding(PocN2, "/slots", "at least one slot is required"))
	}
	for i, s := range d.Slots {
		if s.Slot == "" || !obligations[s.Obligation] || !allGates(s.AppliesTo) {
			out = append(out, finding(PocN2, fmt.Sprintf("/slots/%d", i), "slot %q with obligation %q and gates %v is not allowed", s.Slot, s.Obligation, s.AppliesTo))
		}
	}
	out = append(out, checkFiles(d.Files)...)
	params, err := lifecycle.ParseOver(c.Defaults, d.Lifecycle)
	if err != nil {
		return &d, params, append(out, finding(PocN2, "/lifecycle", "%v", err))
	}
	for g, m := range params.Degraded {
		if (m != "continue" && m != "reject") || !gates[g] {
			out = append(out, finding(PocN2, "/lifecycle/degraded/"+g, "degraded mode %q is not allowed", m))
		}
	}
	return &d, params, append(out, params.Validate()...)
}

// allGates reports whether every gate is a known applies_to value.
func allGates(gs []string) bool {
	for _, g := range gs {
		if !gates[g] {
			return false
		}
	}
	return true
}

// checkFiles validates the structure of each file entry, including D-PVP-1 and content xor source.
func checkFiles(files []fileDecl) []*codes.Error {
	var out []*codes.Error
	seen := map[string]bool{}
	for i, f := range files {
		ptr := fmt.Sprintf("/files/%d", i)
		clean := path.Clean(f.Path)
		switch {
		case f.Path == "" || clean != f.Path || strings.HasPrefix(f.Path, "/") || strings.HasPrefix(clean, ".."):
			out = append(out, finding(PocN2, ptr+"/path", "path %q is not a clean relative path", f.Path))
		case f.Path == pkgv.SlotsPath || f.Path == "manifest.json" || seen[f.Path]:
			out = append(out, finding(PocN2, ptr+"/path", "path %q is reserved or repeated", f.Path))
		}
		seen[f.Path] = true
		if f.MediaType == "" || !roles[f.Role] {
			out = append(out, finding(PocN2, ptr, "media_type and a role in json-schema, shacl, context or other are required; role %q", f.Role))
		}
		if (len(f.Content) == 0) == (f.Source == "") {
			out = append(out, finding(PocN2, ptr, "a file carries exactly one of content and source"))
		}
		if f.MediaType == "application/ld+json" && f.URL == "" {
			out = append(out, finding(PocN2, ptr+"/url", "application/ld+json files require url (D-PVP-1)"))
		}
		if !allGates(f.AppliesTo) {
			out = append(out, finding(PocN2, ptr+"/applies_to", "unknown gate in %v", f.AppliesTo))
		}
	}
	return out
}

// n3 checks that every json-schema file compiles, resolving sources inside the namespace.
func n3(d *draft, ctx app.CompileContext) []*codes.Error {
	var out []*codes.Error
	for i, f := range d.Files {
		if f.Role != pkgv.RoleJSONSchema {
			continue
		}
		b, err := fileBytes(f, ctx)
		if err != nil {
			continue
		}
		if _, err := schemas.Compile(b); err != nil {
			out = append(out, finding(PocN3, fmt.Sprintf("/files/%d", i), "%s does not compile: %v", f.Path, err))
		}
	}
	return out
}

// n5 checks the coherence of slots, files and sources.
func n5(d *draft, ctx app.CompileContext) []*codes.Error {
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
		if f.Slot != "" && !declared[f.Slot] {
			out = append(out, finding(PocN5, fmt.Sprintf("/files/%d/slot", i), "file %s binds undeclared slot %q", f.Path, f.Slot))
		}
		if f.Source != "" {
			if _, err := fileBytes(f, ctx); err != nil {
				out = append(out, finding(PocN5, fmt.Sprintf("/files/%d/source", i), "source %s is not a revision of this namespace: %v", f.Source, err))
			}
		}
	}
	for i, s := range d.Slots {
		if s.Obligation != "optional" && !used[s.Slot] {
			out = append(out, finding(PocN5, fmt.Sprintf("/slots/%d", i), "slot %s has no artefact", s.Slot))
		}
	}
	return out
}

// Compile produces the files, their manifest entries and profile/slots.json.
func (c Compiler) Compile(doc []byte, ctx app.CompileContext) (domain.Compiled, error) {
	d, params, errs := c.parse(doc)
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
		b, err := fileBytes(f, ctx)
		if err != nil {
			return domain.Compiled{}, err
		}
		files[f.Path] = b
		entries = append(entries, pkgv.File{Path: f.Path, MediaType: f.MediaType, Digest: ni.Sum(b), Role: f.Role, Slot: f.Slot, AppliesTo: sorted(f.AppliesTo), URL: f.URL, Source: f.Source})
	}
	var slots []pkgv.Slot
	for _, s := range d.Slots {
		slots = append(slots, pkgv.Slot{Slot: s.Slot, Obligation: s.Obligation, AppliesTo: sorted(s.AppliesTo)})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	sb, err := canon.Marshal(slots)
	if err != nil {
		return domain.Compiled{}, err
	}
	files[pkgv.SlotsPath] = sb
	entries = append(entries, pkgv.File{Path: pkgv.SlotsPath, MediaType: "application/json", Digest: ni.Sum(sb), Role: pkgv.RoleOther})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return domain.Compiled{Version: v, Class: class, Params: params, Profiles: d.Profiles, Entries: entries, Files: files}, nil
}

// sorted returns a sorted copy of a gate list, or nil when empty.
func sorted(gs []string) []string {
	if len(gs) == 0 {
		return nil
	}
	out := append([]string{}, gs...)
	sort.Strings(out)
	return out
}

// fileBytes returns the bytes of a file: a JSON string as text, any other JSON value in canonical form, or a vocabulary revision.
func fileBytes(f fileDecl, ctx app.CompileContext) ([]byte, error) {
	if f.Source != "" {
		if ctx.Vocab == nil {
			return nil, errors.New("no vocabulary reader")
		}
		return ctx.Vocab(f.Source)
	}
	var s string
	if err := json.Unmarshal(f.Content, &s); err == nil {
		return []byte(s), nil
	}
	return canon.Transform(f.Content)
}
