// Term index of an OdrlProfile: expands CURIEs, assigns compact aliases and gathers created and adopted terms.
// Every emitter reads this single index, so the artefacts cannot disagree about what a term is (§5, §13.1).
package dsl

import (
	"sort"
	"strconv"
	"strings"
)

// Entry is one term of a profile, created or adopted.
type Entry struct {
	IRI         string
	CURIE       string
	Alias       string
	Kind        string
	Status      string
	Created     *Term
	Signature   *Signature
	Enforcement *Enforcement
	Ptr         string
}

// Index is the resolved view of a profile.
type Index struct {
	P        *OdrlProfile
	Doc      *Document
	Prefix   string
	TermNS   string
	Entries  []*Entry
	byIRI    map[string]*Entry
	prefixes map[string]string
}

// NewIndex builds the index of a profile document and reports N2 naming findings.
func NewIndex(d *Document, fs *Findings) *Index {
	p := d.Odrl
	ix := &Index{P: p, Doc: d, byIRI: map[string]*Entry{}, prefixes: map[string]string{}}
	for k, v := range Reserved {
		ix.prefixes[k] = v
	}
	ix.Prefix = firstPrefix(d, p.Namespaces)
	for k, v := range p.Namespaces {
		ix.prefixes[k] = v
		if p.Profile.ID == v || p.Profile.ID+"#" == v || p.Profile.ID+"/" == v {
			fs.add(d, "N2", Error, CodeProfileIDIsNamespace, "/namespaces/"+k, "profile id %s must not be the namespace %s", p.Profile.ID, v)
		}
	}
	ix.TermNS = p.Namespaces[ix.Prefix]
	for i := range p.Terms {
		t := &p.Terms[i]
		ptr := "/terms/" + itoa(i)
		e := &Entry{IRI: ix.TermNS + t.ID, CURIE: ix.Prefix + ":" + t.ID, Alias: t.ID, Kind: t.Kind, Status: t.State(),
			Created: t, Signature: t.Signature, Enforcement: t.Enforcement, Ptr: ptr}
		if _, dup := ix.byIRI[e.IRI]; dup {
			fs.add(d, "N2", Error, CodeTermDupID, ptr+"/id", "term id %s is repeated", t.ID)
			continue
		}
		ix.add(e)
	}
	for _, cat := range Categories {
		for i, a := range p.Adopted[cat] {
			ptr := "/adopted/" + cat + "/" + itoa(i)
			iri, ok := ix.Expand(a.Term)
			if !ok {
				fs.add(d, "N2", Error, CodePrefixUndeclared, ptr, "prefix of %s is not declared", a.Term)
				continue
			}
			if _, dup := ix.byIRI[iri]; dup {
				fs.add(d, "N2", Error, CodeTermAdoptedCollision, ptr, "%s is both created and adopted, or adopted twice", a.Term)
				continue
			}
			if (a.Signature != nil || a.Enforcement != nil) && cat != "left_operands" {
				fs.add(d, "N2", Error, CodeSchema, ptr, "signature and enforcement are only allowed on adopted left_operands")
			}
			ix.add(&Entry{IRI: iri, CURIE: a.Term, Alias: localName(a.Term), Kind: categoryKind[cat], Status: "active",
				Signature: a.Signature, Enforcement: a.Enforcement, Ptr: ptr})
		}
	}
	ix.checkAliases(fs)
	return ix
}

// add registers an entry.
func (ix *Index) add(e *Entry) {
	ix.Entries = append(ix.Entries, e)
	ix.byIRI[e.IRI] = e
}

// checkAliases forbids two terms with the same compact alias, which the context could not tell apart.
func (ix *Index) checkAliases(fs *Findings) {
	seen := map[string]*Entry{}
	for _, e := range ix.Entries {
		if o, ok := seen[e.Alias]; ok {
			fs.add(ix.Doc, "N2", Error, CodeAliasCollision, e.Ptr, "%s and %s share the compact alias %q", o.CURIE, e.CURIE, e.Alias)
			continue
		}
		seen[e.Alias] = e
	}
}

// Expand turns a CURIE into an IRI with the declared and reserved prefixes.
func (ix *Index) Expand(curie string) (string, bool) {
	pfx, local, ok := strings.Cut(curie, ":")
	if !ok {
		return "", false
	}
	ns, ok := ix.prefixes[pfx]
	return ns + local, ok
}

// Lookup returns the entry of a CURIE, if the profile has it.
func (ix *Index) Lookup(curie string) (*Entry, bool) {
	iri, ok := ix.Expand(curie)
	if !ok {
		return nil, false
	}
	e, ok := ix.byIRI[iri]
	return e, ok
}

// OfKind returns the entries of a kind ordered by IRI, optionally skipping retired ones.
func (ix *Index) OfKind(kind string, skipRetired bool) []*Entry {
	var out []*Entry
	for _, e := range ix.Entries {
		if e.Kind == kind && !(skipRetired && e.Status == "retired") {
			out = append(out, e)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].IRI < out[j].IRI })
	return out
}

// Signed returns the left operands with a signature, ordered by IRI.
func (ix *Index) Signed() []*Entry {
	var out []*Entry
	for _, e := range ix.OfKind("left_operand", false) {
		if e.Signature != nil {
			out = append(out, e)
		}
	}
	return out
}

// Token returns the directory name of the profile: the last segment of its id.
func (ix *Index) Token() string { return lastSegment(ix.P.Profile.ID) }

// firstPrefix returns the namespace prefix declared first in the source, which is the term namespace.
func firstPrefix(d *Document, ns map[string]string) string {
	best, line := "", 0
	for k := range ns {
		l := d.Lines["/namespaces/"+escapePtr(k)]
		if best == "" || l < line || (l == line && k < best) {
			best, line = k, l
		}
	}
	return best
}

// localName returns the part of a CURIE after the colon.
func localName(curie string) string {
	_, l, _ := strings.Cut(curie, ":")
	return l
}

// lastSegment returns the last non-empty path segment of an IRI.
func lastSegment(iri string) string {
	s := strings.TrimRight(iri, "/#")
	if i := strings.LastIndexAny(s, "/#"); i >= 0 {
		return s[i+1:]
	}
	return s
}

// itoa formats an index for pointers.
func itoa(i int) string { return strconv.Itoa(i) }
