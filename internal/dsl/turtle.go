// Minimal deterministic Turtle writer: prefixed names when the local part is safe, IRIs otherwise, escaped literals.
// All Turtle artefacts go through it so output is byte-stable for the same input (§13, determinism).
package dsl

import (
	"regexp"
	"sort"
	"strings"
)

// safeLocal matches local names that need no escaping in a Turtle prefixed name.
var safeLocal = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_-]*$`)

// turtle collects prefixes and statements.
type turtle struct {
	prefixes map[string]string
	b        strings.Builder
}

// newTurtle starts a document with the given prefixes.
func newTurtle(prefixes map[string]string) *turtle {
	return &turtle{prefixes: prefixes}
}

// ref renders an IRI as a prefixed name when possible.
func (t *turtle) ref(iri string) string {
	best, bestNS := "", ""
	for p, ns := range t.prefixes {
		if strings.HasPrefix(iri, ns) && len(ns) > len(bestNS) && safeLocal.MatchString(iri[len(ns):]) {
			best, bestNS = p, ns
		}
	}
	if best != "" {
		return best + ":" + iri[len(bestNS):]
	}
	return "<" + iri + ">"
}

// lit renders a plain string literal.
func lit(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`).Replace(s) + `"`
}

// langLits renders a LangMap as language-tagged literals ordered by tag.
func langLits(m LangMap) []string {
	tags := make([]string, 0, len(m))
	for k := range m {
		tags = append(tags, k)
	}
	sort.Strings(tags)
	out := make([]string, 0, len(tags))
	for _, k := range tags {
		out = append(out, lit(m[k])+"@"+k)
	}
	return out
}

// subject writes one subject block; each predicate maps to its already rendered objects.
func (t *turtle) subject(s string, pairs ...[2]any) {
	var lines []string
	for _, p := range pairs {
		objs := p[1].([]string)
		if len(objs) == 0 {
			continue
		}
		lines = append(lines, "    "+p[0].(string)+" "+strings.Join(objs, ", "))
	}
	if len(lines) == 0 {
		return
	}
	t.b.WriteString(s + "\n" + strings.Join(lines, " ;\n") + " .\n\n")
}

// raw appends text verbatim.
func (t *turtle) raw(s string) { t.b.WriteString(s) }

// bytes renders the prefixes followed by the statements.
func (t *turtle) bytes() []byte {
	keys := make([]string, 0, len(t.prefixes))
	for k := range t.prefixes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString("@prefix " + k + ": <" + t.prefixes[k] + "> .\n")
	}
	b.WriteString("\n")
	b.WriteString(t.b.String())
	return []byte(b.String())
}

// p builds a predicate–objects pair for subject.
func p(pred string, objs ...string) [2]any {
	var keep []string
	for _, o := range objs {
		if o != "" {
			keep = append(keep, o)
		}
	}
	return [2]any{pred, keep}
}

// list renders an RDF collection.
func list(items []string) string { return "( " + strings.Join(items, " ") + " )" }

// refs renders several IRIs.
func (t *turtle) refs(iris []string) []string {
	out := make([]string, 0, len(iris))
	for _, i := range iris {
		out = append(out, t.ref(i))
	}
	return out
}

// sortedStrings returns a sorted copy.
func sortedStrings(xs []string) []string {
	out := append([]string(nil), xs...)
	sort.Strings(out)
	return out
}
