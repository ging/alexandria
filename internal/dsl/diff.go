// Package dsl implements level N4: diff of a profile or space against its previous release and the minimum SemVer bump of §14.
// Terms are never removed; retiring requires a previous deprecation; a released version never changes content.
package dsl

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/ging/alexandria/internal/core/semver"
)

// BumpInfo summarizes the minimum SemVer bump required relative to the previous release.
type BumpInfo struct {
	Level            string   `json:"level"`
	SuggestedVersion string   `json:"suggested_version,omitempty"`
	Reasons          []string `json:"reasons,omitempty"`
}

// Bump levels, ordered.
const (
	bumpNone = iota
	bumpPatch
	bumpMinor
	bumpMajor
)

// bumpName renders a bump level.
var bumpName = map[int]string{bumpNone: "none", bumpPatch: "PATCH", bumpMinor: "MINOR", bumpMajor: "MAJOR"}

// nextSemver increments v by the given bump level (treating bumpNone as PATCH).
func nextSemver(v semver.Version, level int) string {
	switch level {
	case bumpMajor:
		return fmt.Sprintf("%d.0.0", v.Major+1)
	case bumpMinor:
		return fmt.Sprintf("%d.%d.0", v.Major, v.Minor+1)
	default:
		return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch+1)
	}
}

// change accumulates the required bump and its reasons.
type change struct {
	level   int
	reasons []string
}

// need raises the required bump with a reason.
func (c *change) need(level int, format string, args ...any) {
	if level > c.level {
		c.level = level
	}
	c.reasons = append(c.reasons, fmt.Sprintf(format, args...))
}

// actualBump compares two versions; a negative result means the version went down.
func actualBump(prev, next string) int {
	a, err1 := semver.Parse(prev)
	b, err2 := semver.Parse(next)
	switch {
	case err1 != nil || err2 != nil || !semver.Less(a, b):
		return -1
	case b.Major > a.Major:
		return bumpMajor
	case b.Minor > a.Minor:
		return bumpMinor
	}
	return bumpPatch
}

// report emits VERSION_BUMP_TOO_LOW when the actual bump is below the required one.
func (c *change) report(d *Document, ptr, prev, next string, fs *Findings) {
	got := actualBump(prev, next)
	if got < 0 {
		fs.add(d, "N4", Error, CodeVersionBumpTooLow, ptr, "version %s does not increase on %s", next, prev)
		return
	}
	if got < c.level {
		fs.add(d, "N4", Error, CodeVersionBumpTooLow, ptr, "%s → %s is a %s bump; the changes require %s: %v", prev, next, bumpName[got], bumpName[c.level], c.reasons)
	}
}

// diffProfiles is N4 between two releases of the same OdrlProfile.
func diffProfiles(prev, next *Index, fs *Findings) {
	c := profileChange(prev, next, fs)
	c.report(next.Doc, "/profile/version", prev.P.Profile.Version, next.P.Profile.Version, fs)
}

// profileChange computes the required SemVer bump between two releases of an OdrlProfile; fs may be nil for pure bump analysis.
func profileChange(prev, next *Index, fs *Findings) *change {
	d, c := next.Doc, &change{}
	if prev.TermNS != next.TermNS || prev.Prefix != next.Prefix {
		if fs != nil {
			fs.add(d, "N4", Error, CodeNamespaceChanged, "/namespaces", "the term namespace is stable and cannot change (%s → %s)", prev.TermNS, next.TermNS)
		}
		c.need(bumpMajor, "term namespace changed")
	}
	pm, nm := prev.P.Profile, next.P.Profile
	if !reflect.DeepEqual(pm.Title, nm.Title) || !reflect.DeepEqual(pm.Description, nm.Description) || pm.Publisher != nm.Publisher {
		c.need(bumpPatch, "profile prose changed")
	}
	if pm.License != nm.License || pm.BaseURL != nm.BaseURL {
		c.need(bumpMinor, "license or base_url changed")
	}
	if prev.P.Rules.ActionsClosed() != next.P.Rules.ActionsClosed() || prev.P.Rules.LeftOperandsClosed() != next.P.Rules.LeftOperandsClosed() {
		if (!prev.P.Rules.ActionsClosed() && next.P.Rules.ActionsClosed()) || (!prev.P.Rules.LeftOperandsClosed() && next.P.Rules.LeftOperandsClosed()) {
			c.need(bumpMajor, "a vocabulary list was closed")
		} else {
			c.need(bumpMinor, "a vocabulary list was opened")
		}
	}
	for _, pe := range prev.Entries {
		ne, ok := next.byIRI[pe.IRI]
		switch {
		case !ok && pe.Created != nil:
			if fs != nil {
				fs.add(d, "N4", Error, CodeTermRemoved, "/terms", "term %s was removed; terms are deprecated and retired, never removed", pe.CURIE)
			}
			c.need(bumpMajor, "term %s removed", pe.CURIE)
		case !ok:
			c.need(bumpMajor, "adoption of %s removed", pe.CURIE)
		default:
			diffEntry(pe, ne, c, fs, d)
		}
	}
	for _, ne := range next.Entries {
		if _, ok := prev.byIRI[ne.IRI]; !ok {
			c.need(bumpMinor, "%s added", ne.CURIE)
		}
	}
	return c
}

// diffEntry compares one term across releases.
func diffEntry(pe, ne *Entry, c *change, fs *Findings, d *Document) {
	if pe.Kind != ne.Kind {
		c.need(bumpMajor, "kind of %s changed", ne.CURIE)
	}
	switch from, to := pe.Status, ne.Status; {
	case from == to:
	case from == "active" && to == "deprecated":
		c.need(bumpMinor, "%s deprecated", ne.CURIE)
	case from == "deprecated" && to == "retired":
		c.need(bumpMajor, "%s retired", ne.CURIE)
	case to == "retired":
		if fs != nil {
			fs.add(d, "N4", Error, CodeRetiredNoDeprecation, ne.Ptr+"/status", "%s must be deprecated in a previous release before it is retired", ne.CURIE)
		}
		c.need(bumpMajor, "%s retired", ne.CURIE)
	default:
		c.need(bumpMinor, "status of %s changed", ne.CURIE)
	}
	if pt, nt := pe.Created, ne.Created; pt != nil && nt != nil {
		if !reflect.DeepEqual(pt.Label, nt.Label) || !reflect.DeepEqual(pt.Definition, nt.Definition) || !reflect.DeepEqual(pt.Note, nt.Note) || pt.ReplacedBy != nt.ReplacedBy {
			c.need(bumpPatch, "prose of %s changed", ne.CURIE)
		}
		if pt.IncludedIn != nt.IncludedIn || pt.SubclassOf != nt.SubclassOf || pt.ExtraTurtle != nt.ExtraTurtle {
			c.need(bumpMinor, "semantics of %s changed", ne.CURIE)
		}
	}
	diffSignature(ne.CURIE, pe.Signature, ne.Signature, c)
	switch pa, na := pe.Enforcement, ne.Enforcement; {
	case pa == nil && na != nil:
		c.need(bumpMinor, "enforcement of %s added", ne.CURIE)
	case pa != nil && (na == nil || *pa != *na):
		c.need(bumpMajor, "enforcement of %s changed or removed", ne.CURIE)
	}
}

// diffSignature classifies a signature change: widening is MINOR, restricting is MAJOR.
func diffSignature(name string, a, b *Signature, c *change) {
	switch {
	case a == nil && b == nil:
		return
	case a == nil:
		c.need(bumpMajor, "signature of %s added", name)
		return
	case b == nil:
		c.need(bumpMinor, "signature of %s removed", name)
		return
	}
	added, removed := setDiff(a.Operators, b.Operators)
	if removed > 0 {
		c.need(bumpMajor, "operator removed from %s", name)
	} else if added > 0 {
		c.need(bumpMinor, "operator added to %s", name)
	}
	added, removed = setDiff(a.Values, b.Values)
	if removed > 0 {
		c.need(bumpMajor, "value removed from %s", name)
	} else if added > 0 {
		c.need(bumpMinor, "value added to %s", name)
	}
	if a.Datatype != b.Datatype || a.Kind() != b.Kind() || a.Scheme != b.Scheme {
		c.need(bumpMajor, "type of %s changed", name)
	}
	if a.AllowReference && !b.AllowReference {
		c.need(bumpMajor, "references forbidden on %s", name)
	} else if !a.AllowReference && b.AllowReference {
		c.need(bumpMinor, "references allowed on %s", name)
	}
}

// setDiff counts the elements added to and removed from a string set.
func setDiff(a, b []string) (added, removed int) {
	in := func(xs []string, x string) bool {
		for _, y := range xs {
			if y == x {
				return true
			}
		}
		return false
	}
	for _, x := range b {
		if !in(a, x) {
			added++
		}
	}
	for _, x := range a {
		if !in(b, x) {
			removed++
		}
	}
	return added, removed
}

// obligationRank orders obligations.
var obligationRank = map[string]int{"optional": 0, "recommended": 1, "mandatory": 2}

// diffSpaces is N4 between two releases of a SpaceProfile with their resolved bindings.
func diffSpaces(prev, next *Document, prevRes, nextRes []*Resolved, fs *Findings) {
	c := spaceChange(prev, next, prevRes, nextRes, true)
	c.report(next, "/space/version", prev.Space.Space.Version, next.Space.Space.Version, fs)
}

// spaceChange computes the required SemVer bump between two SpaceProfile releases.
func spaceChange(prev, next *Document, prevRes, nextRes []*Resolved, checkInternalVer bool) *change {
	c := &change{}
	ps, ns := prev.Space, next.Space
	if !reflect.DeepEqual(ps.Space.Title, ns.Space.Title) || !reflect.DeepEqual(ps.Space.Description, ns.Space.Description) {
		c.need(bumpPatch, "space prose changed")
	}
	if ps.Space.Operator != ns.Space.Operator || ps.Space.BaseURL != ns.Space.BaseURL || !jsonEqual(ps.Lifecycle, ns.Lifecycle) {
		c.need(bumpMinor, "operator, base_url or lifecycle changed")
	}
	pb, nb := bySlot(prevRes), bySlot(nextRes)
	declaredNext := map[string]bool{}
	for _, b := range ns.Bindings {
		declaredNext[b.Slot] = true
	}
	for _, slot := range sortedKeys(pb, nb) {
		p, n := pb[slot], nb[slot]
		switch {
		case p == nil && n.Binding.Obligation == "mandatory":
			c.need(bumpMajor, "mandatory binding %s added", slot)
		case p == nil:
			c.need(bumpMinor, "binding %s added", slot)
		case n == nil && declaredNext[slot]:
			// Slot is still declared in next but failed N3 resolution; avoid a false "removed" finding.
			continue
		case n == nil && p.Binding.Obligation == "mandatory":
			c.need(bumpMajor, "mandatory binding %s removed", slot)
		case n == nil:
			c.need(bumpMinor, "binding %s removed", slot)
		default:
			diffBinding(slot, p, n, c, checkInternalVer)
		}
	}
	return c
}

// diffBinding compares the binding of one slot across space releases.
func diffBinding(slot string, p, n *Resolved, c *change, checkInternalVer bool) {
	pb, nb := p.Binding, n.Binding
	if pb.Profile != nb.Profile || pb.Source != nb.Source {
		c.need(bumpMajor, "profile of slot %s changed", slot)
		return
	}
	switch pr, nr := obligationRank[pb.Obligation], obligationRank[nb.Obligation]; {
	case nr > pr:
		c.need(bumpMajor, "obligation of %s raised", slot)
	case nr < pr && pb.Obligation == "mandatory":
		c.need(bumpMajor, "obligation of %s lowered from mandatory", slot)
	case nr < pr:
		c.need(bumpMinor, "obligation of %s lowered", slot)
	}
	if !reflect.DeepEqual(pb.Conformance, nb.Conformance) {
		if nb.Obligation == "mandatory" && len(nb.Conformance) > len(pb.Conformance) {
			c.need(bumpMajor, "gate added to mandatory slot %s", slot)
		} else {
			c.need(bumpMinor, "gates of %s changed", slot)
		}
	}
	if p.Version != n.Version {
		if pb.Source == "vendored" {
			c.need(bumpMajor, "vendored %s moved to %s", slot, n.Version)
		} else if checkInternalVer {
			if b := actualBump(p.Version, n.Version); b > 0 {
				c.need(b, "profile of %s moved %s → %s", slot, p.Version, n.Version)
			} else {
				c.need(bumpMajor, "profile of %s went back to %s", slot, n.Version)
			}
		}
	}
}

// computeBump calculates the minimum next space version and reasons relative to the previous release.
func computeBump(r *Result, prevProfiles []*Index, prevSpace *Document, opt Options) *BumpInfo {
	if r.Space == nil || prevSpace == nil || prevSpace.Space == nil {
		return nil
	}
	prevV, err := semver.Parse(prevSpace.Space.Space.Version)
	if err != nil {
		return nil
	}
	var discard Findings
	prevRes := validateSpace(prevSpace, prevProfiles, opt, &discard)
	c := spaceChange(prevSpace, r.Space, prevRes, r.Bindings, false)
	for _, ix := range latestByID(r.Profiles) {
		if prev := latestOfID(prevProfiles, ix.P.Profile.ID); prev != nil {
			pc := profileChange(prev, ix, nil)
			if pc.level > c.level {
				c.level = pc.level
			}
			c.reasons = append(c.reasons, pc.reasons...)
		}
	}
	eff := c.level
	if eff == bumpNone {
		eff = bumpPatch
	}
	return &BumpInfo{Level: bumpName[eff], SuggestedVersion: nextSemver(prevV, eff), Reasons: dedupe(c.reasons)}
}

// latestByID returns the highest-version index per profile ID in order of ID.
func latestByID(ixs []*Index) []*Index {
	best := map[string]*Index{}
	bestV := map[string]semver.Version{}
	for _, ix := range ixs {
		id := ix.P.Profile.ID
		v, err := semver.Parse(ix.P.Profile.Version)
		if prev, ok := best[id]; !ok || (err == nil && semver.Less(bestV[id], v)) || prev == nil {
			best[id] = ix
			if err == nil {
				bestV[id] = v
			}
		}
	}
	ids := make([]string, 0, len(best))
	for id := range best {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := make([]*Index, 0, len(ids))
	for _, id := range ids {
		out = append(out, best[id])
	}
	return out
}

// latestOfID returns the highest-version release of profileID in ixs, or nil.
func latestOfID(ixs []*Index, profileID string) *Index {
	var best *Index
	var bestV semver.Version
	for _, ix := range ixs {
		if ix.P.Profile.ID != profileID {
			continue
		}
		v, err := semver.Parse(ix.P.Profile.Version)
		if best == nil || (err == nil && semver.Less(bestV, v)) {
			best = ix
			if err == nil {
				bestV = v
			}
		}
	}
	return best
}

// bySlot indexes resolved bindings by slot.
func bySlot(rs []*Resolved) map[string]*Resolved {
	out := map[string]*Resolved{}
	for _, r := range rs {
		out[r.Binding.Slot] = r
	}
	return out
}

// sortedKeys returns the union of the keys of two maps in order.
func sortedKeys(a, b map[string]*Resolved) []string {
	set := map[string]bool{}
	for k := range a {
		set[k] = true
	}
	for k := range b {
		set[k] = true
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// jsonEqual compares two JSON texts semantically.
func jsonEqual(a, b json.RawMessage) bool {
	var x, y any
	_ = json.Unmarshal(a, &x)
	_ = json.Unmarshal(b, &y)
	return reflect.DeepEqual(x, y)
}
