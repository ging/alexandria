// Package dsl provides the emitters of context.jsonld and schema.json: one alias per term, and a self-contained Draft 2020-12 schema over the compact policy.
// The schema does not $ref the DSP schemas remotely, so gates never resolve anything against the network (§13.1, D6).
package dsl

import (
	"encoding/json"
	"strings"

	"github.com/ging/alexandria/internal/core/canon"
)

// dspDefined are the compact names the DSP 2025-1 context already defines; the profile context must not redefine them.
var dspDefined = map[string]bool{"use": true}

func init() {
	for _, o := range odrlOperators {
		dspDefined[o] = true
	}
}

// ContextURL returns the URL with which messages reference the profile context (D-PVP-1).
func ContextURL(ix *Index) string {
	return strings.TrimRight(ix.P.Profile.BaseURL, "/") + "/context.jsonld"
}

// emitContext renders context.jsonld: profile prefixes, then an alias per created and adopted term.
func emitContext(ix *Index) []byte {
	ctx := map[string]any{"@version": 1.1}
	for k, v := range ix.P.Namespaces {
		ctx[k] = v
	}
	for _, e := range ix.Entries {
		if dspDefined[e.Alias] && strings.HasPrefix(e.IRI, Reserved["odrl"]) {
			continue
		}
		if strings.HasPrefix(e.IRI, Reserved["odrl"]) {
			ctx[e.Alias] = e.IRI
		} else {
			ctx[e.Alias] = e.CURIE
		}
	}
	b, _ := canon.Marshal(map[string]any{"@context": ctx})
	return b
}

// names returns every compact spelling of a term: alias, CURIE and full IRI.
func names(e *Entry) []string {
	out := []string{e.Alias, e.CURIE, e.IRI}
	if strings.HasPrefix(e.IRI, Reserved["odrl"]) {
		out[1] = "odrl:" + e.Alias
	}
	return dedupe(out)
}

// dedupe removes repeated strings, keeping order.
func dedupe(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

// spellings of the twelve ODRL operators and the Core actions.
func odrlSpellings(local string) []string {
	return []string{local, "odrl:" + local, Reserved["odrl"] + local}
}

// emitSchema renders schema.json.
func emitSchema(ix *Index) []byte {
	id := ix.P.Profile.ID
	ref := func(name string) map[string]any { return map[string]any{"$ref": "#/$defs/" + name} }
	arrayOf := func(name string) map[string]any { return map[string]any{"type": "array", "items": ref(name)} }
	profileIDs := []string{id, id + "/" + ix.P.Profile.Version}
	defs := map[string]any{
		"Rule": map[string]any{"type": "object", "properties": map[string]any{
			"action": ref("Action"), "constraint": arrayOf("Constraint"), "duty": arrayOf("Rule"),
		}},
		"Constraint": map[string]any{"oneOf": []any{ref("LogicalConstraint"), ref("AtomicConstraint")}},
		"LogicalConstraint": map[string]any{
			"type": "object", "not": map[string]any{"required": []string{"leftOperand"}},
			"anyOf":      []any{req("and"), req("or"), req("xone"), req("andSequence")},
			"properties": map[string]any{"and": arrayOf("Constraint"), "or": arrayOf("Constraint"), "xone": arrayOf("Constraint"), "andSequence": arrayOf("Constraint")},
		},
		"AtomicConstraint": atomic(ix),
		"Action":           action(ix),
	}
	s := map[string]any{
		"$schema":  "https://json-schema.org/draft/2020-12/schema",
		"$id":      strings.TrimRight(ix.P.Profile.BaseURL, "/") + "/" + ix.P.Profile.Version + "/schema.json",
		"title":    ix.P.Profile.Title["en"] + " " + ix.P.Profile.Version + " - ODRL policy (DSP 2025-1 compact form)",
		"type":     "object",
		"required": []string{"profile"},
		"properties": map[string]any{
			"profile": map[string]any{"anyOf": []any{
				map[string]any{"enum": profileIDs},
				map[string]any{"type": "array", "contains": map[string]any{"enum": profileIDs}},
			}},
			"permission": arrayOf("Rule"), "prohibition": arrayOf("Rule"), "obligation": arrayOf("Rule"),
		},
		"$defs": defs,
	}
	b, _ := json.MarshalIndent(s, "", "  ")
	return append(b, '\n')
}

// req builds a schema requiring one property.
func req(name string) map[string]any { return map[string]any{"required": []string{name}} }

// action is the schema of an action: a closed enumeration of compact names, or any string when open.
func action(ix *Index) map[string]any {
	if !ix.P.Rules.ActionsClosed() {
		return map[string]any{"anyOf": []any{map[string]any{"type": "string"}, map[string]any{"type": "object"}}}
	}
	var enum []string
	for _, e := range ix.OfKind("action", true) {
		enum = append(enum, names(e)...)
	}
	for _, c := range odrlCore {
		enum = append(enum, odrlSpellings(c)...)
	}
	enum = dedupe(enum)
	return map[string]any{"anyOf": []any{
		map[string]any{"enum": enum},
		map[string]any{"type": "object", "required": []string{"@id"}, "properties": map[string]any{"@id": map[string]any{"enum": enum}}},
	}}
}

// atomic is the schema of an atomic constraint with one conditional per signature (§9, §13.1).
func atomic(ix *Index) map[string]any {
	out := map[string]any{
		"type": "object", "required": []string{"leftOperand", "operator"},
		"anyOf": []any{req("rightOperand"), req("rightOperandReference")},
	}
	if ix.P.Rules.LeftOperandsClosed() {
		var enum []string
		for _, e := range ix.OfKind("left_operand", true) {
			enum = append(enum, names(e)...)
		}
		var lo any = map[string]any{"enum": dedupe(enum)}
		if len(enum) == 0 {
			lo = false
		}
		out["properties"] = map[string]any{"leftOperand": lo}
	}
	var conds []any
	for _, e := range ix.Signed() {
		if e.Status == "retired" {
			continue
		}
		conds = append(conds, map[string]any{
			"if":   map[string]any{"required": []string{"leftOperand"}, "properties": map[string]any{"leftOperand": map[string]any{"enum": names(e)}}},
			"then": signatureSchema(ix, e.Signature),
		})
	}
	if len(conds) > 0 {
		out["allOf"] = conds
	}
	return out
}

// signatureSchema compiles one signature: operators, right-operand form and references.
func signatureSchema(ix *Index, s *Signature) map[string]any {
	var ops []string
	for _, o := range s.Operators {
		if stdOps[o] {
			ops = append(ops, odrlSpellings(o)...)
		} else if e, ok := ix.Lookup(o); ok {
			ops = append(ops, names(e)...)
		}
	}
	item := valueSchema(ix, s)
	ro := item
	if s.HasSetOperator() {
		ro = map[string]any{"anyOf": []any{item, map[string]any{"type": "array", "minItems": 1, "items": item}}}
	}
	then := map[string]any{"properties": map[string]any{"operator": map[string]any{"enum": dedupe(ops)}, "rightOperand": ro}}
	if !s.AllowReference {
		then["required"] = []string{"rightOperand"}
		then["not"] = req("rightOperandReference")
	}
	return then
}

// valueSchema is the compact JSON-LD form of one right-operand value.
func valueSchema(ix *Index, s *Signature) map[string]any {
	if s.Kind() == "iri" {
		idSchema := map[string]any{"type": "string"}
		if len(s.Values) > 0 {
			var enum []string
			for _, v := range s.Values {
				iri, _ := ix.Expand(v)
				enum = append(enum, v, iri)
			}
			idSchema = map[string]any{"enum": dedupe(enum)}
		}
		return map[string]any{"type": "object", "required": []string{"@id"}, "properties": map[string]any{"@id": idSchema}}
	}
	typed := map[string]any{"type": "object", "required": []string{"@value"}, "properties": map[string]any{"@value": map[string]any{"type": "string"}}}
	if s.Datatype != "" {
		full, _ := ix.Expand(s.Datatype)
		typed["required"] = []string{"@value", "@type"}
		typed["properties"].(map[string]any)["@type"] = map[string]any{"enum": []string{s.Datatype, full}}
	}
	if len(s.Values) > 0 {
		typed["properties"].(map[string]any)["@value"] = map[string]any{"enum": s.Values}
	}
	alts := []any{typed}
	switch s.Datatype {
	case "", "xsd:string":
		plain := map[string]any{"type": "string"}
		if len(s.Values) > 0 {
			plain = map[string]any{"enum": s.Values}
		}
		alts = append(alts, plain)
	case "xsd:integer":
		alts = append(alts, map[string]any{"type": "integer"})
	case "xsd:decimal", "xsd:double", "xsd:float":
		alts = append(alts, map[string]any{"type": "number"})
	case "xsd:boolean":
		alts = append(alts, map[string]any{"type": "boolean"})
	}
	if len(alts) == 1 {
		return typed
	}
	return map[string]any{"anyOf": alts}
}
