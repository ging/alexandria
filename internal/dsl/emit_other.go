// Package dsl provides the emitters of prof.ttl, spec.html, cedar-mapping.json and cedar.schema for a profile (§13.1, §13.4).
// The PROF manifest lists every generated artefact with its role and media type.
package dsl

import (
	"encoding/json"
	"fmt"
	"html"
	"sort"
	"strings"
)

// PROF roles of the artefacts, and the eu: roles for the Cedar projections.
const (
	profRole = "http://www.w3.org/ns/dx/prof/role/"
	alxRole  = "https://w3id.org/alexandria/ns/role/"
)

// artefactSpec describes one profile artefact for prof.ttl.
type artefactSpec struct {
	file, frag, role, mediaType, conformsTo string
}

// profileArtefacts lists the artefacts of a profile; Cedar ones only when Cedar is enabled and some term has enforcement.
func profileArtefacts(ix *Index) []artefactSpec {
	out := []artefactSpec{
		{"spec.html", "spec", profRole + "specification", "text/html", ""},
		{"vocab.ttl", "vocab", profRole + "vocabulary", "text/turtle", "http://www.w3.org/2002/07/owl"},
		{"shapes.ttl", "shapes", profRole + "validation", "text/turtle", "https://www.w3.org/TR/shacl/"},
		{"schema.json", "jsonschema", profRole + "validation", "application/schema+json", "https://json-schema.org/draft/2020-12/schema"},
		{"context.jsonld", "context", profRole + "mapping", "application/ld+json", "https://www.w3.org/TR/json-ld11/"},
	}
	if emitsCedar(ix) {
		out = append(out,
			artefactSpec{"cedar-mapping.json", "cedarmapping", alxRole + "cedar-mapping", "application/json", ""},
			artefactSpec{"cedar.schema", "cedarschema", alxRole + "cedar-schema", "text/plain", ""})
	}
	return out
}

// hasEnforcement reports whether any signed left operand has enforcement.
func hasEnforcement(ix *Index) bool {
	for _, e := range ix.Signed() {
		if e.Enforcement != nil {
			return true
		}
	}
	return false
}

// emitProf renders the PROF manifest of a profile.
func emitProf(ix *Index) []byte {
	m := ix.P.Profile
	base := strings.TrimRight(m.BaseURL, "/") + "/"
	t := newTurtle(map[string]string{
		"prof": "http://www.w3.org/ns/dx/prof/", "role": profRole, "alxrole": alxRole,
		"dct": Reserved["dct"], "owl": Reserved["owl"], "xsd": Reserved["xsd"], "r": base,
	})
	core := "http://www.w3.org/ns/odrl/2/core"
	t.subject("<"+core+">", p("a", "dct:Standard"), p("dct:title", lit("ODRL Core Profile 2.2")))
	var res []string
	for _, a := range profileArtefacts(ix) {
		res = append(res, t.ref(base+a.frag))
	}
	t.subject("<"+m.ID+">",
		p("a", "prof:Profile"),
		p("dct:title", langLits(m.Title)...),
		p("dct:description", langLits(m.Description)...),
		p("prof:isProfileOf", "<"+core+">"),
		p("prof:hasToken", lit(ix.Token())),
		p("owl:versionInfo", lit(m.Version)),
		p("dct:publisher", lit(m.Publisher)),
		p("dct:license", "<"+m.License+">"),
		p("prof:hasResource", res...))
	for _, a := range profileArtefacts(ix) {
		pairs := [][2]any{
			p("a", "prof:ResourceDescriptor"), p("prof:hasRole", t.ref(a.role)),
			p("prof:hasArtifact", "<"+base+a.file+">"),
			p("dct:format", "<https://www.iana.org/assignments/media-types/"+a.mediaType+">"),
		}
		if a.conformsTo != "" {
			pairs = append(pairs, p("dct:conformsTo", "<"+a.conformsTo+">"))
		}
		t.subject(t.ref(base+a.frag), pairs...)
	}
	return t.bytes()
}

// emitSpec renders a human-readable specification of a profile.
func emitSpec(ix *Index) []byte {
	m := ix.P.Profile
	var b strings.Builder
	esc := html.EscapeString
	fmt.Fprintf(&b, "<!DOCTYPE html>\n<html lang=\"en\"><head><meta charset=\"utf-8\"><title>%s %s</title></head><body>\n", esc(m.Title["en"]), esc(m.Version))
	fmt.Fprintf(&b, "<h1>%s</h1>\n<p><code>%s</code> version %s, published by %s under <a href=\"%s\">%s</a>.</p>\n",
		esc(m.Title["en"]), esc(m.ID), esc(m.Version), esc(m.Publisher), esc(m.License), esc(m.License))
	if d := m.Description["en"]; d != "" {
		fmt.Fprintf(&b, "<p>%s</p>\n", esc(d))
	}
	fmt.Fprintf(&b, "<p>Actions: %s. Left operands: %s.</p>\n", closure(ix.P.Rules.ActionsClosed()), closure(ix.P.Rules.LeftOperandsClosed()))
	b.WriteString("<h2>Terms</h2>\n<table><tr><th>Term</th><th>Kind</th><th>Status</th><th>Label</th><th>Definition</th></tr>\n")
	for _, e := range sortedEntries(ix.Entries) {
		label, def := "", "adopted"
		if e.Created != nil {
			label, def = e.Created.Label["en"], e.Created.Definition["en"]
		}
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%s</td><td>%s</td></tr>\n", esc(e.CURIE), esc(e.Kind), esc(e.Status), esc(label), esc(def))
	}
	b.WriteString("</table>\n<h2>Signatures</h2>\n<table><tr><th>Left operand</th><th>Operators</th><th>Right operand</th><th>References</th></tr>\n")
	for _, e := range ix.Signed() {
		s := e.Signature
		ro := s.Kind()
		if s.Datatype != "" {
			ro += " " + s.Datatype
		}
		if len(s.Values) > 0 {
			ro += " in {" + strings.Join(s.Values, ", ") + "}"
		}
		if s.Scheme != "" {
			ro += " in scheme " + s.Scheme
		}
		fmt.Fprintf(&b, "<tr><td><code>%s</code></td><td>%s</td><td>%s</td><td>%v</td></tr>\n", esc(e.CURIE), esc(strings.Join(s.Operators, ", ")), esc(ro), s.AllowReference)
	}
	b.WriteString("</table>\n</body></html>\n")
	return []byte(b.String())
}

// closure names a closure policy.
func closure(closed bool) string {
	if closed {
		return "closed"
	}
	return "open"
}

// cedarType maps a signature to its Cedar type (§13.4).
func cedarType(s *Signature) string {
	switch s.Datatype {
	case "xsd:integer":
		return "Long"
	case "xsd:boolean":
		return "Bool"
	case "xsd:dateTime":
		return "datetime"
	}
	if s.HasSetOperator() {
		return "Set<String>"
	}
	return "String"
}

// cedarOps maps ODRL operators to Cedar expressions.
var cedarOps = map[string]string{"eq": "==", "neq": "!=", "lt": "<", "lteq": "<=", "gt": ">", "gteq": ">=", "isAnyOf": "contains", "isNoneOf": "!contains", "isAllOf": "containsAll"}

// emitCedarMapping renders cedar-mapping.json.
func emitCedarMapping(ix *Index) []byte {
	type mapping struct {
		LeftOperand string            `json:"left_operand"`
		CedarPath   string            `json:"cedar_path"`
		Type        string            `json:"type"`
		Operators   map[string]string `json:"operators"`
		Values      map[string]string `json:"values,omitempty"`
		PDPLevel    string            `json:"pdp_level,omitempty"`
		EvalTiming  string            `json:"eval_timing,omitempty"`
	}
	var ms []mapping
	for _, e := range ix.Signed() {
		if e.Enforcement == nil {
			continue
		}
		m := mapping{
			LeftOperand: e.IRI, CedarPath: e.Enforcement.CedarPath, Type: cedarType(e.Signature), Operators: map[string]string{},
			PDPLevel: e.Enforcement.PDPLevel, EvalTiming: e.Enforcement.EvalTiming,
		}
		for _, o := range e.Signature.Operators {
			if c, ok := cedarOps[o]; ok {
				m.Operators[operatorIRI(ix, o)] = c
			}
		}
		if e.Signature.Kind() == "iri" && len(e.Signature.Values) > 0 {
			m.Values = map[string]string{}
			for _, v := range e.Signature.Values {
				iri, _ := ix.Expand(v)
				m.Values[iri] = localName(v)
			}
		}
		ms = append(ms, m)
	}
	b, _ := json.MarshalIndent(map[string]any{"profile": ix.P.Profile.ID + "/" + ix.P.Profile.Version, "mappings": ms}, "", "  ")
	return append(b, '\n')
}

// emitCedarSchema renders a Cedar schema: one action per profile action with the typed attributes of enforcement paths.
func emitCedarSchema(ix *Index) []byte {
	attrs := map[string]map[string]string{"context": {}, "resource": {}, "principal": {}}
	for _, e := range ix.Signed() {
		if e.Enforcement == nil {
			continue
		}
		root, rest, _ := strings.Cut(e.Enforcement.CedarPath, ".")
		attrs[root][strings.ReplaceAll(rest, ".", "_")] = cedarType(e.Signature)
	}
	record := func(m map[string]string) string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, k+"?: "+m[k])
		}
		return "{ " + strings.Join(parts, ", ") + " }"
	}
	entity := func(name string, m map[string]string) string {
		if len(m) == 0 {
			return "  entity " + name + ";\n"
		}
		return "  entity " + name + " = " + record(m) + ";\n"
	}
	var b strings.Builder
	ns := cedarIdent(ix.Token())
	fmt.Fprintf(&b, "// Cedar schema generated from %s %s; verify that the Cedar version in use supports datetime.\n", ix.P.Profile.ID, ix.P.Profile.Version)
	fmt.Fprintf(&b, "namespace %s {\n%s%s", ns, entity("Party", attrs["principal"]), entity("Asset", attrs["resource"]))
	var actions []string
	for _, e := range ix.OfKind("action", true) {
		actions = append(actions, e.Alias)
	}
	actions = append(actions, odrlCore...)
	for _, a := range dedupe(actions) {
		fmt.Fprintf(&b, "  action %q appliesTo { principal: [Party], resource: [Asset], context: %s };\n", a, record(attrs["context"]))
	}
	b.WriteString("}\n")
	return []byte(b.String())
}

// cedarIdent turns a token into a Cedar identifier.
func cedarIdent(s string) string {
	var b strings.Builder
	for i, r := range s {
		ok := r == '_' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (i > 0 && r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		return "Profile"
	}
	return strings.ToUpper(out[:1]) + out[1:]
}
