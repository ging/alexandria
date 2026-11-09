// Emitter of shapes.ttl following §13.3: closed action and left-operand lists, deprecation warnings and one shape per signature.
// Signature shapes use the conditional pattern sh:or ( not-X | rules ), validated over the expanded graph.
package dsl

import "strings"

// ruleTargets are the properties whose objects are rules.
const ruleTargets = "sh:targetObjectsOf odrl:permission, odrl:prohibition, odrl:obligation, odrl:duty"

// shapesNS is the namespace of the generated shapes.
func shapesNS(ix *Index) string { return strings.TrimRight(ix.P.Profile.BaseURL, "/") + "/shapes#" }

// emitShapes renders shapes.ttl.
func emitShapes(ix *Index) []byte {
	pre := map[string]string{"sh": "http://www.w3.org/ns/shacl#", "odrl": Reserved["odrl"], "xsd": Reserved["xsd"], "skos": Reserved["skos"], "gen": shapesNS(ix)}
	for k, v := range ix.P.Namespaces {
		pre[k] = v
	}
	t := newTurtle(pre)
	actions := refsOf(t, ix.OfKind("action", true))
	if ix.P.Rules.ActionsClosed() {
		t.raw("gen:ActionShape a sh:NodeShape ;\n  " + ruleTargets + " ;\n" +
			"  sh:message \"Action not allowed by profile " + ix.Token() + ".\" ;\n" +
			"  sh:property [ sh:path odrl:action ; sh:minCount 1 ; sh:maxCount 1 ; sh:nodeKind sh:IRI ;\n" +
			"    sh:in " + list(append(append([]string{}, actions...), "odrl:use", "odrl:transfer")) + " ] .\n\n")
		t.raw("gen:ActionCoreShape a sh:NodeShape ;\n  " + ruleTargets + " ;\n" +
			"  sh:message \"ODRL Core actions are ignored by profile " + ix.Token() + ".\" ;\n" +
			"  sh:property [ sh:path odrl:action ; sh:severity sh:Warning ;\n    sh:in " + list(actions) + " ] .\n\n")
	}
	if ix.P.Rules.LeftOperandsClosed() {
		t.raw("gen:LeftOperandShape a sh:NodeShape ;\n  sh:targetSubjectsOf odrl:leftOperand ;\n" +
			"  sh:message \"LeftOperand not defined by profile " + ix.Token() + ".\" ;\n" +
			"  sh:property [ sh:path odrl:leftOperand ; sh:minCount 1 ; sh:maxCount 1 ;\n    sh:in " + list(refsOf(t, ix.OfKind("left_operand", true))) + " ] .\n\n")
	}
	emitDeprecated(t, ix, "action", "Action", ruleTargets, "odrl:action")
	emitDeprecated(t, ix, "left_operand", "LeftOperand", "sh:targetSubjectsOf odrl:leftOperand", "odrl:leftOperand")
	for _, e := range ix.Signed() {
		if e.Status != "retired" {
			emitSigShape(t, ix, e)
		}
	}
	return t.bytes()
}

// refsOf renders the IRIs of entries.
func refsOf(t *turtle, es []*Entry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, t.ref(e.IRI))
	}
	return out
}

// emitDeprecated adds a Warning shape for deprecated terms of a kind (§13.3, status: deprecated).
func emitDeprecated(t *turtle, ix *Index, kind, name, target, path string) {
	var dep []*Entry
	for _, e := range ix.OfKind(kind, false) {
		if e.Status == "deprecated" {
			dep = append(dep, e)
		}
	}
	if len(dep) == 0 {
		return
	}
	t.raw("gen:Deprecated" + name + "Shape a sh:NodeShape ;\n  " + target + " ;\n" +
		"  sh:property [ sh:path " + path + " ; sh:severity sh:Warning ;\n" +
		"    sh:message \"Deprecated term of profile " + ix.Token() + ".\" ;\n" +
		"    sh:not [ sh:in " + list(refsOf(t, dep)) + " ] ] .\n\n")
}

// emitSigShape renders the shape of one signature.
func emitSigShape(t *turtle, ix *Index, e *Entry) {
	s := e.Signature
	var ops []string
	for _, o := range s.Operators {
		ops = append(ops, t.ref(operatorIRI(ix, o)))
	}
	ro := "sh:path odrl:rightOperand"
	if !s.AllowReference {
		ro += " ; sh:minCount 1"
	}
	if !s.HasSetOperator() {
		ro += " ; sh:maxCount 1"
	}
	switch {
	case s.Kind() == "iri":
		ro += " ; sh:nodeKind sh:IRI"
		if len(s.Values) > 0 {
			var vs []string
			for _, v := range s.Values {
				iri, _ := ix.Expand(v)
				vs = append(vs, t.ref(iri))
			}
			ro += " ; sh:in " + list(vs)
		}
		if s.Scheme != "" {
			sc, _ := ix.Expand(s.Scheme)
			ro += " ; sh:node [ sh:property [ sh:path skos:inScheme ; sh:hasValue " + t.ref(sc) + " ] ]"
		}
	default:
		ro += " ; sh:nodeKind sh:Literal"
		if s.Datatype != "" {
			dt, _ := ix.Expand(s.Datatype)
			ro += " ; sh:datatype " + t.ref(dt)
		}
		if len(s.Values) > 0 {
			var vs []string
			for _, v := range s.Values {
				vs = append(vs, lit(v))
			}
			ro += " ; sh:in " + list(vs)
		}
	}
	props := "      sh:property [ sh:path odrl:operator ; sh:minCount 1 ; sh:maxCount 1 ; sh:in " + list(ops) + " ] ;\n" +
		"      sh:property [ " + ro + " ]"
	if !s.AllowReference {
		props += " ;\n      sh:property [ sh:path odrl:rightOperandReference ; sh:maxCount 0 ]"
	}
	lo := t.ref(e.IRI)
	t.raw("gen:Sig_" + e.Alias + " a sh:NodeShape ;\n  sh:targetSubjectsOf odrl:leftOperand ;\n" +
		"  sh:message \"Constraint " + e.CURIE + ": signature not respected.\" ;\n" +
		"  sh:or (\n    [ sh:not [ sh:path odrl:leftOperand ; sh:hasValue " + lo + " ] ]\n    [\n" + props + "\n    ]\n  ) .\n\n")
}
