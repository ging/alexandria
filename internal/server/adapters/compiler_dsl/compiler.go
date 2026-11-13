// Package compiler_dsl is the ProfileCompiler adapter over the alexandria.profiles/v1 language: a draft is a YAML stream with one SpaceProfile and its OdrlProfiles.
// Language roles outside the manifest enum become "other", and slot obligations go to profile/slots.json (agent decision, REQ-HUB-001, DP-6).
package compiler_dsl

import (
	"errors"
	"sort"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/dsl"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// CodeBundleWithoutSpace is the PoC code of a bundle without SpaceProfile (outside the F5 catalog).
const CodeBundleWithoutSpace codes.Code = "POC_DSL_BUNDLE_WITHOUT_SPACE"

// SourceName is the package path of the draft source (§16 sources).
const SourceName = "sources/draft.yaml"

// manifestRoles are the roles of §3.7; any other language role is published as "other".
var manifestRoles = map[string]bool{pkgv.RoleJSONSchema: true, pkgv.RoleSHACL: true, pkgv.RoleContext: true}

// Compiler compiles YAML drafts; Vendor resolves vendored bindings from local copies.
type Compiler struct{ Vendor dsl.Vendor }

var _ app.ProfileCompiler = Compiler{}

// options builds the language options for a hub context.
func (c Compiler) options(ctx app.CompileContext) dsl.Options {
	opt := dsl.Options{Vendor: c.Vendor}
	if len(ctx.Previous) > 0 {
		opt.Previous = []dsl.Source{{Name: "previous.yaml", Data: ctx.Previous}}
	}
	return opt
}

// Validate runs N1–N3, or N1–N5 when level is 5, and the bundle shape check.
func (c Compiler) Validate(doc []byte, level int, ctx app.CompileContext) []*codes.Error {
	r := dsl.Compile(c.options(ctx), dsl.Source{Name: "draft.yaml", Data: doc})
	var out []*codes.Error
	for _, f := range r.Findings.Errors().Sorted() {
		if level < 5 && (f.Level == "N4" || f.Level == "N5") {
			continue
		}
		out = append(out, &codes.Error{Code: codes.Code(f.Code), Path: f.Path, Line: f.Line, Detail: f.Document + ": " + f.Message})
	}
	if len(out) > 0 {
		return out
	}
	if r.Space == nil {
		return []*codes.Error{{Code: CodeBundleWithoutSpace, Detail: "a draft is a YAML stream with exactly one SpaceProfile and the OdrlProfiles it binds"}}
	}
	sp := r.Space.Space.Space
	if v, err := semver.Parse(sp.Version); level >= 5 && err == nil {
		if _, ok := lifecycle.ClassOf(v, ctx.Latest); !ok {
			return []*codes.Error{{Code: dsl.CodeVersionBumpTooLow, Path: "/space/version", Line: r.Space.Line("/space/version"), Detail: "space version " + sp.Version + " is not greater than the latest release " + ctx.Latest.String()}}
		}
	}
	return nil
}

// Compile produces the package files: every binding's artefacts, the space artefacts, the source and profile/slots.json.
func (c Compiler) Compile(doc []byte, ctx app.CompileContext) (domain.Compiled, error) {
	r := dsl.Compile(c.options(ctx), dsl.Source{Name: "draft.yaml", Data: doc})
	if !r.OK() || r.Space == nil {
		return domain.Compiled{}, errors.New("compiler_dsl: draft does not compile")
	}
	v, err := semver.Parse(r.Space.Space.Space.Version)
	if err != nil {
		return domain.Compiled{}, err
	}
	class, ok := lifecycle.ClassOf(v, ctx.Latest)
	if !ok {
		return domain.Compiled{}, errors.New("compiler_dsl: space version does not increase")
	}
	files := map[string][]byte{SourceName: doc}
	entries := []pkgv.File{{Path: SourceName, MediaType: "application/yaml", Digest: ni.Sum(doc), Role: pkgv.RoleOther}}
	for _, a := range r.Artifacts {
		role := a.Role
		if !manifestRoles[role] {
			role = pkgv.RoleOther
		}
		files[a.Path] = a.Bytes
		entries = append(entries, pkgv.File{Path: a.Path, MediaType: a.MediaType, Digest: ni.Sum(a.Bytes), Role: role, Slot: a.Slot, AppliesTo: nonEmpty(a.AppliesTo), URL: a.URL})
	}
	var slots []pkgv.Slot
	var profiles []string
	for _, b := range r.Bindings {
		gates := []string{}
		for _, cf := range b.Binding.Conformance {
			gates = append(gates, cf.AppliesTo)
		}
		sort.Strings(gates)
		slots = append(slots, pkgv.Slot{Slot: b.Binding.Slot, Obligation: b.Binding.Obligation, AppliesTo: gates})
	}
	for _, ps := range r.Lock.Profiles {
		profiles = append(profiles, ps...)
	}
	sort.Strings(profiles)
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	sb, err := canon.Marshal(slots)
	if err != nil {
		return domain.Compiled{}, err
	}
	files[pkgv.SlotsPath] = sb
	entries = append(entries, pkgv.File{Path: pkgv.SlotsPath, MediaType: "application/json", Digest: ni.Sum(sb), Role: pkgv.RoleOther})
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	params := r.Lock.Lifecycle
	if params.Degraded == nil {
		params.Degraded = map[string]string{}
	}
	return domain.Compiled{Version: v, Class: class, Params: params, Profiles: dedupe(profiles), Entries: entries, Files: files}, nil
}

// nonEmpty returns nil for an empty gate list so applies_to is omitted.
func nonEmpty(gs []string) []string {
	if len(gs) == 0 {
		return nil
	}
	return append([]string{}, gs...)
}

// dedupe removes adjacent duplicates of a sorted list.
func dedupe(s []string) []string {
	var out []string
	for i, x := range s {
		if i == 0 || x != s[i-1] {
			out = append(out, x)
		}
	}
	return out
}
