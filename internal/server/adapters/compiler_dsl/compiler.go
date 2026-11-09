// ProfileCompiler adapter over the eunomia.profiles/v1 language: a draft is a YAML stream with one SpaceProfile and its OdrlProfiles.
// Each slot's artefacts enter the package with the gates of its binding, so the space is a verifiable stack of specifications (REQ-HUB-001, REQ-HUB-004, DP-6).
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

// Adapter-level codes for the bundle shape and the hub's single space (not in the F5 catalog).
const (
	CodeBundleWithoutSpace codes.Code = "BUNDLE_WITHOUT_SPACE"
	CodeSpaceMismatch      codes.Code = "SPACE_MISMATCH"
)

// SourceName is the package path of the draft source (§16 sources).
const SourceName = "sources/draft.yaml"

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

// Validate runs N1–N3, or N1–N5 when level is 5, and the hub checks of the bundle.
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
	if ctx.Space != "" && sp.ID != ctx.Space {
		return []*codes.Error{{Code: CodeSpaceMismatch, Path: "/space/id", Line: r.Space.Line("/space/id"), Detail: "this hub serves " + ctx.Space + ", not " + sp.ID}}
	}
	if v, err := semver.Parse(sp.Version); level >= 5 && err == nil {
		if _, ok := lifecycle.ClassOf(v, ctx.Latest); !ok {
			return []*codes.Error{{Code: dsl.CodeVersionBumpTooLow, Path: "/space/version", Line: r.Space.Line("/space/version"), Detail: "space version " + sp.Version + " is not greater than the latest release " + ctx.Latest.String()}}
		}
	}
	return nil
}

// Compile produces the package: artefacts of every binding, the space artefacts, the source, manifest.json and space-lock.json.
func (c Compiler) Compile(doc []byte, ctx app.CompileContext) (domain.Compiled, error) {
	r := dsl.Compile(c.options(ctx), dsl.Source{Name: "draft.yaml", Data: doc})
	if !r.OK() || r.Space == nil {
		return domain.Compiled{}, errors.New("compiler_dsl: draft does not compile")
	}
	sp := r.Space.Space.Space
	v, err := semver.Parse(sp.Version)
	if err != nil {
		return domain.Compiled{}, err
	}
	class, ok := lifecycle.ClassOf(v, ctx.Latest)
	if !ok {
		return domain.Compiled{}, errors.New("compiler_dsl: space version does not increase")
	}
	files := map[string][]byte{SourceName: doc}
	entries := []pkgv.File{{Path: SourceName, MediaType: "application/yaml", Digest: ni.Sum(doc), Role: "source", AppliesTo: []string{}}}
	for _, a := range r.Artifacts {
		files[a.Path] = a.Bytes
		applies := append([]string{}, a.AppliesTo...)
		entries = append(entries, pkgv.File{Path: a.Path, MediaType: a.MediaType, Digest: ni.Sum(a.Bytes), Role: a.Role, Slot: a.Slot, AppliesTo: applies, URL: a.URL})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	var slots []pkgv.Slot
	for _, b := range r.Bindings {
		slots = append(slots, pkgv.Slot{Slot: b.Binding.Slot, Obligation: b.Binding.Obligation})
	}
	sort.Slice(slots, func(i, j int) bool { return slots[i].Slot < slots[j].Slot })
	manifest, err := canon.Marshal(pkgv.Manifest{Space: sp.ID, Version: v.String(), Files: entries, Slots: slots})
	if err != nil {
		return domain.Compiled{}, err
	}
	r.Lock.Manifest = string(ni.Sum(manifest))
	return domain.Compiled{Space: sp.ID, Version: v, Class: class, Params: r.Lock.Lifecycle, Manifest: manifest,
		SpaceLock: r.Lock.JSON(), Files: files, Entries: entries}, nil
}
