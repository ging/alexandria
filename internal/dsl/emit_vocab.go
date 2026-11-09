// Emitter of vocab.ttl: ontology, SKOS collections, created terms in their §8.5 RDF form and reified signatures (§9.3).
// Adopted terms are only listed as members; nothing is asserted about IRIs the profile does not own (§8.4).
package dsl

import (
	"sort"
	"strings"
)

// collections names the per-kind SKOS collections of the Best Practice.
var collections = []struct{ kind, frag, label string }{
	{"action", "actions", "Actions"},
	{"left_operand", "leftOperands", "Constraint Left Operands"},
	{"operator", "operators", "Constraint Operators"},
	{"right_operand", "rightOperands", "Constraint Right Operands"},
	{"conflict_term", "conflictTerms", "Conflict Terms"},
	{"party_function", "partyFunctions", "Party Functions"},
	{"asset_relation", "assetRelations", "Asset Relations"},
	{"logical_operand", "logicalOperands", "Logical Operands"},
	{"rule_subclass", "ruleSubclasses", "Rule Subclasses"},
	{"policy_subclass", "policySubclasses", "Policy Subclasses"},
	{"constraint_subclass", "constraintSubclasses", "Constraint Subclasses"},
}

// individualClass is the ODRL class of each individual-like kind.
var individualClass = map[string]string{"left_operand": "LeftOperand", "operator": "Operator", "right_operand": "RightOperand", "conflict_term": "ConflictTerm"}

// propertyParent is the ODRL super-property of each property-like kind.
var propertyParent = map[string]string{"party_function": "function", "asset_relation": "relation", "logical_operand": "operand"}

// odrlSiblings are the ODRL subclasses a new subclass is made disjoint with.
var odrlSiblings = map[string][]string{
	"rule_subclass":   {"Permission", "Prohibition", "Duty"},
	"policy_subclass": {"Agreement", "Assertion", "Offer", "Privacy", "Request", "Ticket"},
}

// vocabPrefixes returns the prefixes used by the profile Turtle artefacts.
func vocabPrefixes(ix *Index) map[string]string {
	out := map[string]string{}
	for _, k := range []string{"odrl", "rdf", "rdfs", "owl", "skos", "dct", "xsd", "sig", "eu"} {
		out[k] = Reserved[k]
	}
	for k, v := range ix.P.Namespaces {
		out[k] = v
	}
	return out
}

// sigIRI is the IRI of the reified signature of a left operand.
func sigIRI(ix *Index, e *Entry) string { return ix.P.Profile.ID + "#sig-" + e.Alias }

// emitVocab renders vocab.ttl.
func emitVocab(ix *Index) []byte {
	t := newTurtle(vocabPrefixes(ix))
	id, meta := ix.P.Profile.ID, ix.P.Profile
	var members, sigs []string
	for _, e := range sortedEntries(ix.Entries) {
		members = append(members, t.ref(e.IRI))
	}
	for _, e := range ix.Signed() {
		sigs = append(sigs, t.ref(sigIRI(ix, e)))
	}
	t.subject("<"+id+">",
		p("a", "owl:Ontology", "skos:Collection"),
		p("rdfs:label", langLits(meta.Title)...),
		p("owl:versionInfo", lit(meta.Version)),
		p("owl:versionIRI", "<"+id+"/"+meta.Version+">"),
		p("dct:creator", lit(meta.Publisher)),
		p("dct:description", langLits(meta.Description)...),
		p("dct:license", "<"+meta.License+">"),
		p("skos:member", members...),
		p("sig:hasSignature", sigs...))
	for _, c := range collections {
		es := ix.OfKind(c.kind, false)
		if len(es) == 0 {
			continue
		}
		var ms []string
		for _, e := range es {
			ms = append(ms, t.ref(e.IRI))
		}
		t.subject("<"+id+"#"+c.frag+">", p("a", "skos:Collection"), p("skos:prefLabel", lit(c.label)+"@en"), p("skos:member", ms...))
	}
	for _, e := range sortedEntries(ix.Entries) {
		if e.Created != nil {
			emitTerm(t, ix, e)
		}
	}
	for _, e := range ix.Signed() {
		emitSignature(t, ix, e)
	}
	t.raw(annotationDeclarations)
	for _, e := range sortedEntries(ix.Entries) {
		if e.Created != nil && e.Created.ExtraTurtle != "" {
			t.raw("# extra_turtle of " + e.CURIE + "\n" + strings.TrimSpace(e.Created.ExtraTurtle) + "\n\n")
		}
	}
	return t.bytes()
}

// emitTerm renders one created term in the form of its kind.
func emitTerm(t *turtle, ix *Index, e *Entry) {
	tm := e.Created
	expand := func(c string) string {
		if c == "" {
			return ""
		}
		iri, _ := ix.Expand(c)
		return t.ref(iri)
	}
	pairs := [][2]any{}
	switch {
	case tm.Kind == "action":
		pairs = append(pairs, p("a", "odrl:Action", "skos:Concept"), p("odrl:includedIn", expand(tm.IncludedIn)))
	case individualClass[tm.Kind] != "":
		pairs = append(pairs, p("a", "odrl:"+individualClass[tm.Kind], "owl:NamedIndividual", "skos:Concept"))
	case propertyParent[tm.Kind] != "":
		pairs = append(pairs, p("a", "rdf:Property", "owl:ObjectProperty"), p("rdfs:subPropertyOf", "odrl:"+propertyParent[tm.Kind]))
		if tm.Kind == "asset_relation" {
			pairs = append(pairs, p("rdfs:domain", "odrl:Rule"), p("rdfs:range", "odrl:Asset"))
		}
	default:
		pairs = append(pairs, p("a", "owl:Class"), p("rdfs:subClassOf", expand(tm.SubclassOf)), p("owl:disjointWith", siblings(t, ix, e)...))
	}
	pairs = append(pairs,
		p("rdfs:isDefinedBy", "<"+ix.P.Profile.ID+">"),
		p("rdfs:label", langLits(tm.Label)...),
		p("skos:definition", langLits(tm.Definition)...),
		p("skos:note", langLits(tm.Note)...),
		p("dct:isReplacedBy", expand(tm.ReplacedBy)))
	if tm.State() != "active" {
		pairs = append(pairs, p("owl:deprecated", "true"), p("eu:status", lit(tm.State())))
	}
	t.subject(t.ref(e.IRI), pairs...)
}

// siblings returns the classes a new subclass is disjoint with: ODRL siblings and created siblings of the same parent.
func siblings(t *turtle, ix *Index, e *Entry) []string {
	var out []string
	parent := e.Created.SubclassOf
	if parent == "odrl:Rule" || parent == "odrl:Policy" {
		for _, s := range odrlSiblings[e.Kind] {
			out = append(out, "odrl:"+s)
		}
	}
	for _, o := range ix.OfKind(e.Kind, false) {
		if o != e && o.Created != nil && o.Created.SubclassOf == parent {
			out = append(out, t.ref(o.IRI))
		}
	}
	return out
}

// emitSignature renders the reified signature of a left operand, with its enforcement in eu:.
func emitSignature(t *turtle, ix *Index, e *Entry) {
	s := e.Signature
	var ops, vals []string
	for _, o := range s.Operators {
		ops = append(ops, t.ref(operatorIRI(ix, o)))
	}
	if s.Kind() == "iri" {
		for _, v := range s.Values {
			iri, _ := ix.Expand(v)
			vals = append(vals, t.ref(iri))
		}
	} else {
		for _, v := range s.Values {
			vals = append(vals, lit(v))
		}
	}
	kind := "sig:Literal"
	if s.Kind() == "iri" {
		kind = "sig:IRI"
	}
	pairs := [][2]any{
		p("a", "sig:Signature"), p("sig:leftOperand", t.ref(e.IRI)), p("sig:operator", ops...),
		p("sig:valueKind", kind), p("sig:value", vals...), p("sig:allowReference", boolLit(s.AllowReference)),
	}
	if s.Datatype != "" {
		dt, _ := ix.Expand(s.Datatype)
		pairs = append(pairs, p("sig:datatype", t.ref(dt)))
	}
	if s.Scheme != "" {
		sc, _ := ix.Expand(s.Scheme)
		pairs = append(pairs, p("sig:scheme", t.ref(sc)))
	}
	if en := e.Enforcement; en != nil {
		pairs = append(pairs, p("eu:cedarPath", lit(en.CedarPath)), p("eu:pdpLevel", optLit(en.PDPLevel)), p("eu:evalTiming", optLit(en.EvalTiming)))
	}
	t.subject(t.ref(sigIRI(ix, e)), pairs...)
}

// operatorIRI expands an ODRL operator name or a profile operator CURIE.
func operatorIRI(ix *Index, o string) string {
	if stdOps[o] {
		return Reserved["odrl"] + o
	}
	iri, _ := ix.Expand(o)
	return iri
}

// boolLit renders a boolean literal.
func boolLit(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// optLit renders a literal or nothing.
func optLit(s string) string {
	if s == "" {
		return ""
	}
	return lit(s)
}

// sortedEntries orders entries by IRI.
func sortedEntries(es []*Entry) []*Entry {
	out := append([]*Entry(nil), es...)
	sort.Slice(out, func(i, j int) bool { return out[i].IRI < out[j].IRI })
	return out
}

// annotationDeclarations keeps the ontology within OWL DL (§6.2, §9.3).
const annotationDeclarations = `sig:Signature a owl:Class .
sig:hasSignature a owl:AnnotationProperty .
sig:leftOperand a owl:AnnotationProperty .
sig:operator a owl:AnnotationProperty .
sig:datatype a owl:AnnotationProperty .
sig:valueKind a owl:AnnotationProperty .
sig:value a owl:AnnotationProperty .
sig:scheme a owl:AnnotationProperty .
sig:allowReference a owl:AnnotationProperty .
eu:cedarPath a owl:AnnotationProperty .
eu:pdpLevel a owl:AnnotationProperty .
eu:evalTiming a owl:AnnotationProperty .
eu:status a owl:AnnotationProperty .
skos:member a owl:AnnotationProperty .
skos:prefLabel a owl:AnnotationProperty .
skos:definition a owl:AnnotationProperty .
skos:note a owl:AnnotationProperty .
dct:creator a owl:AnnotationProperty .
dct:description a owl:AnnotationProperty .
dct:license a owl:AnnotationProperty .
dct:isReplacedBy a owl:AnnotationProperty .
skos:Collection a owl:Class .
skos:Concept a owl:Class .

`
