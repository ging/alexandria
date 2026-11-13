// Package dsl implements the SpaceProfile levels N2 and N3: one binding per slot, gates for mandatory bindings, lifecycle invariants and binding resolution.
// Internal bindings resolve against OdrlProfile releases in the bundle; vendored ones against a local vendor store (§11, §12).
package dsl

import (
	"bytes"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
)

// Resolved is a binding together with what it resolved to.
type Resolved struct {
	Binding  Binding
	Ptr      string
	Profile  *Index
	Vendored *Vendored
	Version  string
}

// Roles returns the artefact roles a resolved binding can offer to its gates.
func (r *Resolved) Roles() map[string]bool {
	out := map[string]bool{}
	switch {
	case r.Profile != nil:
		out["shacl"], out["json-schema"] = true, true
		for _, e := range r.Profile.Signed() {
			if cedarEnabled && e.Enforcement != nil {
				out["cedar-mapping"], out["cedar-schema"] = true, true
			}
		}
	case r.Vendored != nil:
		for _, a := range r.Vendored.Artifacts {
			out[a.Role] = true
		}
	}
	return out
}

// SpaceLifecycle reads the space lifecycle on top of the §2.10 defaults; conformance_required is ignored.
func SpaceLifecycle(sp *SpaceProfile) (lifecycle.Params, error) {
	raw := sp.Lifecycle
	if len(raw) > 0 {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			return lifecycle.Params{}, err
		}
		delete(m, "conformance_required")
		raw, _ = json.Marshal(m)
	}
	return lifecycle.ParseOver(lifecycle.Defaults(), raw)
}

// validateSpace runs N2 and N3 on the space and returns its resolved bindings.
func validateSpace(d *Document, profiles []*Index, vendor Vendor, fs *Findings) []*Resolved {
	sp := d.Space
	if len(sp.Lifecycle) > 0 && bytes.Contains(sp.Lifecycle, []byte(`"conformance_required"`)) {
		fs.add(d, "N2", Warning, CodeComplianceIgnored, "/lifecycle/conformance_required", "conformance is out of scope of this compiler; the value is ignored")
	}
	if p, err := SpaceLifecycle(sp); err != nil {
		fs.add(d, "N2", Error, CodeSchema, "/lifecycle", "%v", err)
	} else {
		for _, e := range p.Validate() {
			fs.add(d, "N2", Error, string(e.Code), e.Path, "%s", e.Detail)
		}
	}
	seen := map[string]bool{}
	var out []*Resolved
	for i, b := range sp.Bindings {
		ptr := "/bindings/" + strconv.Itoa(i)
		if seen[b.Slot] {
			fs.add(d, "N2", Error, CodeSlotDup, ptr+"/slot", "slot %s has more than one binding", b.Slot)
			continue
		}
		seen[b.Slot] = true
		if b.Obligation == "mandatory" && len(b.Conformance) == 0 {
			fs.add(d, "N2", Error, CodeMandatoryUnverifiable, ptr, "a mandatory binding needs at least one gate")
		}
		for _, v := range b.Via {
			if v == b.Profile {
				fs.add(d, "N2", Error, CodeViaCycle, ptr+"/via", "via contains the bound profile itself")
			}
		}
		if r := resolve(d, ptr, b, profiles, vendor, fs); r != nil {
			out = append(out, r)
		}
	}
	return out
}

// resolve is N3 for one binding.
func resolve(d *Document, ptr string, b Binding, profiles []*Index, vendor Vendor, fs *Findings) *Resolved {
	r := &Resolved{Binding: b, Ptr: ptr}
	switch b.Source {
	case "internal":
		if b.Slot != "odrl" {
			fs.add(d, "N3", Error, CodeBindingBaseMismatch, ptr+"/source", "slot %s cannot be internal: the language only defines OdrlProfile documents", b.Slot)
			return nil
		}
		if len(b.Via) > 0 {
			fs.add(d, "N3", Error, CodeBindingBaseMismatch, ptr+"/via", "an internal OdrlProfile is a direct profile of ODRL Core; via must be empty")
		}
		r.Profile = pickProfile(b, profiles)
		if r.Profile == nil {
			fs.add(d, "N3", Error, CodeBindingUnresolved, ptr, "no release of %s satisfies %s in the bundle", b.Profile, b.Version)
			return nil
		}
		r.Version = r.Profile.P.Profile.Version
	case "vendored":
		id := strings.TrimPrefix(b.Version, "=")
		if vendor != nil {
			r.Vendored, _ = vendor.Lookup(b.Slot, b.Profile, id)
		}
		if r.Vendored == nil {
			fs.add(d, "N3", Error, CodeBindingUnresolved, ptr, "no vendored copy of %s %s for slot %s", b.Profile, id, b.Slot)
			return nil
		}
		if len(r.Vendored.IsProfileOf) > 0 && len(b.Via) > 0 && !sameChain(b.Via, r.Vendored.IsProfileOf) {
			fs.add(d, "N3", Error, CodeBindingBaseMismatch, ptr+"/via", "via %v does not match prof:isProfileOf %v", b.Via, r.Vendored.IsProfileOf)
		}
		r.Version = id
	}
	roles := r.Roles()
	for j, c := range b.Conformance {
		if !roles[c.Role] {
			fs.add(d, "N3", Error, CodeArtifactMissing, ptr+"/conformance/"+strconv.Itoa(j), "%s %s produces no %s artefact", b.Profile, r.Version, c.Role)
		}
	}
	return r
}

// sameChain reports whether via lists the intermediate profiles of the declared chain.
func sameChain(via, chain []string) bool {
	if len(via) > len(chain) {
		return false
	}
	for i := range via {
		if via[i] != chain[i] {
			return false
		}
	}
	return true
}

// pickProfile returns the greatest profile release that satisfies the binding range (§11.4).
func pickProfile(b Binding, profiles []*Index) *Index {
	var best *Index
	var bestV semver.Version
	for _, ix := range profiles {
		if ix.P.Profile.ID != b.Profile {
			continue
		}
		v, err := semver.Parse(ix.P.Profile.Version)
		if err != nil || !Satisfies(b.Version, v) {
			continue
		}
		if best == nil || semver.Less(bestV, v) {
			best, bestV = ix, v
		}
	}
	return best
}

// Satisfies evaluates =X.Y.Z, ^X.Y, ~X.Y and bare versions; missing parts are wildcards.
func Satisfies(rng string, v semver.Version) bool {
	op := ""
	if rng != "" && strings.ContainsRune("=^~", rune(rng[0])) {
		op, rng = rng[:1], rng[1:]
	}
	parts := strings.Split(rng, ".")
	nums := make([]uint64, len(parts))
	for i, p := range parts {
		n, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return false
		}
		nums[i] = n
	}
	for len(nums) < 3 {
		nums = append(nums, 0)
	}
	lo := semver.Version{Major: nums[0], Minor: nums[1], Patch: nums[2]}
	if v.Pre != "" || semver.Less(v, lo) {
		return false
	}
	switch op {
	case "^":
		if lo.Major > 0 {
			return v.Major == lo.Major
		}
		if lo.Minor > 0 || len(parts) > 1 {
			return v.Major == 0 && v.Minor == lo.Minor
		}
		return v.Major == 0
	case "~":
		if len(parts) == 1 {
			return v.Major == lo.Major
		}
		return v.Major == lo.Major && v.Minor == lo.Minor
	}
	switch len(parts) {
	case 1:
		return v.Major == lo.Major
	case 2:
		return v.Major == lo.Major && v.Minor == lo.Minor
	}
	return semver.Compare(v, lo) == 0
}
