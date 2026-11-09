// Package dsl implements the compilation pipeline: load (N1), index and validate (N2, N3), diff against the previous release (N4), emit, check outputs (N5).
// A bundle is one SpaceProfile plus the OdrlProfiles it binds; each slot's artefacts carry the gates of its binding (§5, §13).
package dsl

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"path"
	"sort"
	"strings"

	"github.com/ging/alexandria/internal/core/canon"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/semver"
)

// Options configure a compilation.
type Options struct {
	Previous []Source
	Vendor   Vendor
}

// Artifact is one generated or vendored file of the output.
type Artifact struct {
	Path      string
	MediaType string
	Role      string
	Slot      string
	AppliesTo []string
	URL       string
	Bytes     []byte
}

// LockArtifact is one artefact of a binding in space-lock.json.
type LockArtifact struct {
	Path      string   `json:"path"`
	Role      string   `json:"role"`
	MediaType string   `json:"media_type"`
	SHA256    string   `json:"sha256"`
	AppliesTo []string `json:"applies_to"`
}

// LockBinding is the resolved form of a binding in space-lock.json (§13.2).
type LockBinding struct {
	Slot       string         `json:"slot"`
	Profile    string         `json:"profile"`
	Source     string         `json:"source"`
	Version    string         `json:"version"`
	Obligation string         `json:"obligation"`
	Artifacts  []LockArtifact `json:"artifacts"`
}

// Lock is space-lock.json; Lifecycle and Profiles are the fields alexandria-client also reads (S7, §4.9).
type Lock struct {
	Space     string              `json:"space"`
	Version   string              `json:"version"`
	Manifest  string              `json:"manifest,omitempty"`
	Lifecycle lifecycle.Params    `json:"lifecycle"`
	Profiles  map[string][]string `json:"profiles,omitempty"`
	Bindings  []LockBinding       `json:"bindings"`
}

// Result is the outcome of a compilation; Artifacts is empty when there are errors.
type Result struct {
	Space     *Document
	Profiles  []*Index
	Bindings  []*Resolved
	Artifacts []Artifact
	Lock      *Lock
	Findings  Findings
}

// OK reports whether the compilation has no errors.
func (r *Result) OK() bool { return len(r.Findings.Errors()) == 0 }

// Compile runs the whole pipeline on a bundle.
func Compile(opt Options, srcs ...Source) *Result {
	r := &Result{}
	docs, fs := Load(srcs...)
	r.Findings = fs
	for _, d := range docs {
		switch {
		case d.Odrl != nil:
			r.Profiles = append(r.Profiles, NewIndex(d, &r.Findings))
		case r.Space != nil:
			r.Findings.add(d, "N2", Error, CodeSchema, "", "a bundle has at most one SpaceProfile; %s already defines one", r.Space.Label())
		default:
			r.Space = d
		}
	}
	prevProfiles, prevSpace := loadPrevious(opt)
	all := append(append([]*Index{}, r.Profiles...), prevProfiles...)
	for _, ix := range r.Profiles {
		validateProfile(ix, all, &r.Findings)
	}
	if r.Space != nil {
		r.Bindings = validateSpace(r.Space, all, opt.Vendor, &r.Findings)
	}
	versioning(r, prevProfiles, prevSpace, opt)
	if !r.OK() {
		return r
	}
	r.emit()
	r.checkOutputs()
	if !r.OK() {
		r.Artifacts, r.Lock = nil, nil
	}
	return r
}

// loadPrevious loads the previous release's sources, ignoring their findings.
func loadPrevious(opt Options) ([]*Index, *Document) {
	if len(opt.Previous) == 0 {
		return nil, nil
	}
	docs, _ := Load(opt.Previous...)
	var ixs []*Index
	var space *Document
	var discard Findings
	for _, d := range docs {
		if d.Odrl != nil {
			ixs = append(ixs, NewIndex(d, &discard))
		} else if space == nil {
			space = d
		}
	}
	return ixs, space
}

// versioning is N4 for every profile of the bundle and for the space.
func versioning(r *Result, prevProfiles []*Index, prevSpace *Document, opt Options) {
	known := append(append([]*Index{}, prevProfiles...), r.Profiles...)
	for _, ix := range r.Profiles {
		if same := sameRelease(ix, prevProfiles); same != nil {
			if !jsonEqual(same.Doc.JSON, ix.Doc.JSON) {
				r.Findings.add(ix.Doc, "N4", Error, CodeProfileVersionChanged, "/profile/version", "%s %s is already released with other content", ix.P.Profile.ID, ix.P.Profile.Version)
			}
			continue
		}
		if prev := predecessor(ix, known); prev != nil {
			diffProfiles(prev, ix, &r.Findings)
		}
	}
	if r.Space == nil || prevSpace == nil || prevSpace.Space == nil {
		return
	}
	if prevSpace.Space.Space.Version == r.Space.Space.Space.Version {
		if !jsonEqual(prevSpace.JSON, r.Space.JSON) {
			r.Findings.add(r.Space, "N4", Error, CodeProfileVersionChanged, "/space/version", "space %s is already released with other content", r.Space.Space.Space.Version)
		}
		return
	}
	var discard Findings
	prevRes := validateSpace(prevSpace, prevProfiles, opt.Vendor, &discard)
	diffSpaces(prevSpace, r.Space, prevRes, r.Bindings, &r.Findings)
}

// sameRelease returns a previous release with the same id and version.
func sameRelease(ix *Index, prev []*Index) *Index {
	for _, p := range prev {
		if p.P.Profile.ID == ix.P.Profile.ID && p.P.Profile.Version == ix.P.Profile.Version {
			return p
		}
	}
	return nil
}

// predecessor returns the greatest known release of the same profile below ix.
func predecessor(ix *Index, known []*Index) *Index {
	v, err := semver.Parse(ix.P.Profile.Version)
	if err != nil {
		return nil
	}
	var best *Index
	var bestV semver.Version
	for _, k := range known {
		kv, err := semver.Parse(k.P.Profile.Version)
		if err != nil || k == ix || k.P.Profile.ID != ix.P.Profile.ID || !semver.Less(kv, v) {
			continue
		}
		if best == nil || semver.Less(bestV, kv) {
			best, bestV = k, kv
		}
	}
	return best
}

// ProfileDir is the directory of a profile's artefacts inside a package.
func ProfileDir(ix *Index) string {
	return path.Join("profiles", "odrl", ix.Token(), ix.P.Profile.Version)
}

// profileFiles renders all artefacts of a profile, keyed by file name.
func profileFiles(ix *Index) map[string][]byte {
	out := map[string][]byte{
		"vocab.ttl": emitVocab(ix), "context.jsonld": emitContext(ix), "shapes.ttl": emitShapes(ix),
		"schema.json": emitSchema(ix), "prof.ttl": emitProf(ix), "spec.html": emitSpec(ix),
	}
	if hasEnforcement(ix) {
		out["cedar-mapping.json"], out["cedar.schema"] = emitCedarMapping(ix), emitCedarSchema(ix)
	}
	return out
}

// fileRoles maps a generated file to its role and media type.
var fileRoles = map[string][2]string{
	"vocab.ttl": {"vocabulary", "text/turtle"}, "context.jsonld": {"mapping", "application/ld+json"},
	"shapes.ttl": {"shacl", "text/turtle"}, "schema.json": {"json-schema", "application/schema+json"},
	"prof.ttl": {"prof", "text/turtle"}, "spec.html": {"specification", "text/html"},
	"cedar-mapping.json": {"cedar-mapping", "application/json"}, "cedar.schema": {"cedar-schema", "text/plain"},
}

// gatesFor returns the gates of a binding that use a role.
func gatesFor(b *Resolved, role string) []string {
	out := []string{}
	if b == nil {
		return out
	}
	for _, c := range b.Binding.Conformance {
		if c.Role == role {
			out = append(out, c.AppliesTo)
		}
	}
	sort.Strings(out)
	return dedupe(out)
}

// emit renders the profile artefacts, the vendored copies and, for a bundle with a space, the space artefacts.
func (r *Result) emit() {
	bound := map[*Index]*Resolved{}
	for _, b := range r.Bindings {
		if b.Profile != nil {
			bound[b.Profile] = b
		}
	}
	for _, ix := range r.Profiles {
		b := bound[ix]
		if r.Space != nil && b == nil {
			continue
		}
		files := profileFiles(ix)
		for _, name := range sortedKeysOf(files) {
			role := fileRoles[name]
			a := Artifact{Path: path.Join(ProfileDir(ix), name), MediaType: role[1], Role: role[0], Slot: "odrl", AppliesTo: gatesFor(b, role[0]), Bytes: files[name]}
			if name == "context.jsonld" {
				a.URL = ContextURL(ix)
			}
			r.Artifacts = append(r.Artifacts, a)
		}
	}
	for _, b := range r.Bindings {
		if v := b.Vendored; v != nil {
			for _, va := range v.Artifacts {
				r.Artifacts = append(r.Artifacts, Artifact{
					Path: path.Join("profiles", b.Binding.Slot, "vendored", b.Version, va.Path), MediaType: va.MediaType,
					Role: va.Role, Slot: b.Binding.Slot, AppliesTo: gatesFor(b, va.Role), URL: va.URL, Bytes: va.Bytes,
				})
			}
		}
	}
	if r.Space == nil {
		return
	}
	r.Artifacts = append(r.Artifacts,
		Artifact{Path: "space/prof.ttl", MediaType: "text/turtle", Role: "prof", AppliesTo: []string{}, Bytes: r.spaceProf()},
		Artifact{Path: "space/shacl-union.ttl", MediaType: "text/turtle", Role: "shacl-union", AppliesTo: []string{}, Bytes: r.shaclUnion()},
		Artifact{Path: "space/spec.html", MediaType: "text/html", Role: "specification", AppliesTo: []string{}, Bytes: r.spaceSpec()})
	r.Lock = r.buildLock()
}

// buildLock assembles space-lock.json from the resolved bindings and their artefacts.
func (r *Result) buildLock() *Lock {
	params, _ := SpaceLifecycle(r.Space.Space)
	l := &Lock{Space: r.Space.Space.Space.ID, Version: r.Space.Space.Space.Version, Lifecycle: params, Profiles: map[string][]string{}}
	for _, b := range r.Bindings {
		lb := LockBinding{Slot: b.Binding.Slot, Profile: b.Binding.Profile, Source: b.Binding.Source, Version: b.Version, Obligation: b.Binding.Obligation, Artifacts: []LockArtifact{}}
		for _, a := range r.Artifacts {
			if a.Slot == b.Binding.Slot {
				sum := sha256.Sum256(a.Bytes)
				lb.Artifacts = append(lb.Artifacts, LockArtifact{Path: a.Path, Role: a.Role, MediaType: a.MediaType, SHA256: hex.EncodeToString(sum[:]), AppliesTo: a.AppliesTo})
			}
		}
		l.Bindings = append(l.Bindings, lb)
		if b.Profile != nil {
			l.Profiles[b.Binding.Slot] = []string{b.Binding.Profile, b.Binding.Profile + "/" + b.Version}
		} else {
			l.Profiles[b.Binding.Slot] = []string{b.Binding.Profile}
		}
	}
	return l
}

// JSON renders a lock in canonical form.
func (l *Lock) JSON() []byte {
	b, _ := canon.Marshal(l)
	return b
}

// spaceProf renders the PROF manifest of the space: one isProfileOf per binding and one descriptor per artefact.
func (r *Result) spaceProf() []byte {
	s := r.Space.Space.Space
	base := strings.TrimRight(s.BaseURL, "/") + "/" + s.Version + "/"
	t := newTurtle(map[string]string{"prof": "http://www.w3.org/ns/dx/prof/", "dct": Reserved["dct"], "owl": Reserved["owl"], "eu": Reserved["eu"]})
	var of, res []string
	for _, b := range r.Bindings {
		of = append(of, "<"+b.Binding.Profile+">")
	}
	for i := range r.Artifacts {
		res = append(res, fmt.Sprintf("<%s#r%d>", s.ID, i))
	}
	t.subject("<"+s.ID+">", p("a", "prof:Profile"), p("dct:title", langLits(s.Title)...), p("owl:versionInfo", lit(s.Version)),
		p("dct:publisher", "<"+s.Operator+">"), p("prof:isProfileOf", sortedStrings(of)...), p("prof:hasResource", res...))
	for i, a := range r.Artifacts {
		t.subject(fmt.Sprintf("<%s#r%d>", s.ID, i), p("a", "prof:ResourceDescriptor"), p("eu:role", lit(a.Role)),
			p("eu:slot", optLit(a.Slot)), p("eu:appliesTo", litsOf(a.AppliesTo)...),
			p("prof:hasArtifact", "<"+base+a.Path+">"), p("dct:format", "<https://www.iana.org/assignments/media-types/"+a.MediaType+">"))
	}
	return t.bytes()
}

// litsOf renders strings as literals.
func litsOf(xs []string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		out = append(out, lit(x))
	}
	return out
}

// shaclUnion concatenates the SHACL artefacts of mandatory bindings, indexed by gate, and flags shape namespace clashes.
func (r *Result) shaclUnion() []byte {
	var b bytes.Buffer
	b.WriteString("# SHACL union of the mandatory bindings, indexed by gate. Each section is a complete Turtle document.\n\n")
	seen := map[string]string{}
	byGate := map[string][]Artifact{}
	for _, a := range r.Artifacts {
		if a.Role != "shacl" {
			continue
		}
		for _, bnd := range r.Bindings {
			if bnd.Binding.Slot == a.Slot && bnd.Binding.Obligation == "mandatory" {
				for _, g := range a.AppliesTo {
					byGate[g] = append(byGate[g], a)
				}
			}
		}
		if a.Slot == "odrl" {
			ns := shapesNSOf(r, a)
			if prev, dup := seen[ns]; dup && prev != a.Path {
				r.Findings.add(r.Space, "N5", Error, CodeShaclUnionConflict, "/bindings", "%s and %s define shapes in the same namespace %s", prev, a.Path, ns)
			}
			seen[ns] = a.Path
		}
	}
	for _, g := range sortedKeysOfArtifacts(byGate) {
		for _, a := range byGate[g] {
			fmt.Fprintf(&b, "# gate %s · slot %s · %s\n", g, a.Slot, a.Path)
			b.Write(a.Bytes)
			b.WriteString("\n")
		}
	}
	return b.Bytes()
}

// shapesNSOf returns the shapes namespace of a generated SHACL artefact.
func shapesNSOf(r *Result, a Artifact) string {
	for _, ix := range r.Profiles {
		if strings.HasPrefix(a.Path, ProfileDir(ix)+"/") {
			return shapesNS(ix)
		}
	}
	return a.Path
}

// spaceSpec renders the table of bindings.
func (r *Result) spaceSpec() []byte {
	s := r.Space.Space.Space
	esc := html.EscapeString
	var b strings.Builder
	fmt.Fprintf(&b, "<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>%s %s</title></head><body>\n<h1>%s</h1>\n", esc(s.Title["en"]), esc(s.Version), esc(s.Title["en"]))
	fmt.Fprintf(&b, "<p><code>%s</code> version %s, governed by <code>%s</code>.</p>\n", esc(s.ID), esc(s.Version), esc(s.Operator))
	b.WriteString("<table><tr><th>Slot</th><th>Profile</th><th>Source</th><th>Version</th><th>Obligation</th><th>Gates</th></tr>\n")
	for _, bd := range r.Bindings {
		var gates []string
		for _, c := range bd.Binding.Conformance {
			gates = append(gates, c.Role+" @ "+c.AppliesTo)
		}
		fmt.Fprintf(&b, "<tr><td>%s</td><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n",
			esc(bd.Binding.Slot), esc(bd.Binding.Profile), esc(bd.Binding.Source), esc(bd.Version), esc(bd.Binding.Obligation), esc(strings.Join(gates, ", ")))
	}
	b.WriteString("</table>\n</body></html>\n")
	return []byte(b.String())
}

// checkOutputs is N5: JSON parses, schemas compile, contexts expand offline and Turtle is well balanced.
func (r *Result) checkOutputs() {
	for _, a := range r.Artifacts {
		var err error
		switch {
		case a.Role == "json-schema":
			_, err = schemas.Compile(a.Bytes)
		case strings.HasSuffix(a.MediaType, "json"):
			if !json.Valid(a.Bytes) {
				err = fmt.Errorf("not valid JSON")
			}
		case a.MediaType == "text/turtle":
			err = checkTurtle(a.Bytes)
		}
		if err == nil && a.Role == "mapping" {
			err = r.checkMapping(a)
		}
		if err != nil {
			r.Findings.add(r.Space, "N5", Error, CodeOutputInvalid, "", "%s: %v", a.Path, err)
		}
	}
}

// checkMapping runs the JSON-LD check on the context of the profile that produced a mapping artefact.
func (r *Result) checkMapping(a Artifact) error {
	for _, ix := range r.Profiles {
		if a.Path == path.Join(ProfileDir(ix), "context.jsonld") {
			return checkContext(ix, a.Bytes)
		}
	}
	return nil
}

// sortedKeysOf returns the keys of a byte map in order.
func sortedKeysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// sortedKeysOfArtifacts returns the keys of an artefact index in order.
func sortedKeysOfArtifacts(m map[string][]Artifact) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
