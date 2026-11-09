// Package dsl implements levels N2 (invariants) and N3 (references) for OdrlProfile documents, including the §9.4 operator–type table.
// Adopted ODRL terms are checked against the local vocabulary copy; other adoptions against profiles in the bundle (§12).
package dsl

import (
	"strings"
)

// orderedTypes are the datatypes that admit lt, lteq, gt and gteq (§9.4).
var orderedTypes = map[string]bool{
	"xsd:integer": true, "xsd:decimal": true, "xsd:double": true, "xsd:float": true,
	"xsd:date": true, "xsd:dateTime": true, "xsd:time": true,
}

// Operator families of §9.4.
var (
	orderedOps = map[string]bool{"lt": true, "lteq": true, "gt": true, "gteq": true}
	iriOps     = map[string]bool{"isA": true, "hasPart": true, "isPartOf": true}
	setOps     = map[string]bool{"isAnyOf": true, "isNoneOf": true, "isAllOf": true}
	stdOps     = map[string]bool{}
)

func init() {
	for _, o := range odrlOperators {
		stdOps[o] = true
	}
}

// Kind returns the effective value kind of a signature: explicit, implied by datatype, or by CURIE values.
func (s *Signature) Kind() string {
	switch {
	case s.ValueKind != "":
		return s.ValueKind
	case s.Datatype != "":
		return "literal"
	case len(s.Values) > 0 && allCURIE(s.Values):
		return "iri"
	case s.Scheme != "":
		return "iri"
	}
	return "literal"
}

// HasSetOperator reports whether the signature admits a multivalued right operand.
func (s *Signature) HasSetOperator() bool {
	for _, o := range s.Operators {
		if setOps[o] {
			return true
		}
	}
	return false
}

// allCURIE reports whether every value looks like a CURIE.
func allCURIE(vs []string) bool {
	for _, v := range vs {
		if !strings.Contains(v, ":") || strings.Contains(v, " ") {
			return false
		}
	}
	return true
}

// validateProfile runs N2 and N3 on one indexed profile; others resolves adoptions from sibling profiles.
func validateProfile(ix *Index, others []*Index, fs *Findings) {
	d := ix.Doc
	ref := func(ptr, curie string) {
		if curie == "" {
			return
		}
		if _, ok := ix.Expand(curie); !ok {
			fs.add(d, "N2", Error, CodePrefixUndeclared, ptr, "prefix of %s is not declared", curie)
		}
	}
	for _, e := range ix.Entries {
		if t := e.Created; t != nil {
			ref(e.Ptr+"/included_in", t.IncludedIn)
			ref(e.Ptr+"/subclass_of", t.SubclassOf)
			ref(e.Ptr+"/replaced_by", t.ReplacedBy)
			if t.Kind == "action" && t.IncludedIn == "" {
				fs.add(d, "N2", Warning, CodeIncludedInMissing, e.Ptr, "action %s has no included_in", t.ID)
			}
			if t.State() != "active" && t.ReplacedBy == "" {
				fs.add(d, "N2", Warning, CodeNoReplacement, e.Ptr, "%s term %s has no replaced_by", t.State(), t.ID)
			}
			if t.ExtraTurtle != "" {
				fs.add(d, "N2", Warning, CodeExtraTurtle, e.Ptr+"/extra_turtle", "extra_turtle is injected as is; this compiler cannot parse Turtle to check it")
			}
		}
		if e.Enforcement != nil && e.Signature == nil {
			fs.add(d, "N2", Error, CodeEnforcementNoSig, e.Ptr, "enforcement of %s requires a signature (§10)", e.CURIE)
		}
		if e.Signature != nil {
			validateSignature(ix, e, fs)
		}
		if e.Created == nil {
			checkAdopted(ix, e, others, fs)
		}
	}
}

// validateSignature applies §9.4 and the value-kind rules of §9.2.
func validateSignature(ix *Index, e *Entry, fs *Findings) {
	d, s, ptr := ix.Doc, e.Signature, e.Ptr+"/signature"
	if s.Datatype != "" && !strings.HasPrefix(s.Datatype, "xsd:") {
		fs.add(d, "N2", Error, CodeSigValueKind, ptr+"/datatype", "datatype %s must be an xsd: type", s.Datatype)
	}
	if s.Datatype != "" && s.ValueKind == "iri" {
		fs.add(d, "N2", Error, CodeSigValueKind, ptr, "a datatype implies value_kind literal")
	}
	if s.Scheme != "" {
		fs.add(d, "N2", Warning, CodeSigScheme, ptr+"/scheme", "scheme compiles to skos:inScheme: the gate must merge vocab.ttl into the data graph, and no term field declares scheme membership")
	}
	for i, o := range s.Operators {
		optr := ptr + "/operators/" + itoa(i)
		switch {
		case orderedOps[o] && s.Datatype == "xsd:duration":
			fs.add(d, "N2", Warning, CodeSigOperatorType, optr, "%s on xsd:duration is only partially ordered", o)
		case orderedOps[o] && !orderedTypes[s.Datatype]:
			fs.add(d, "N2", Error, CodeSigOperatorType, optr, "%s requires an ordered datatype, not %q", o, s.Datatype)
		case iriOps[o] && s.Kind() != "iri":
			fs.add(d, "N2", Error, CodeSigOperatorType, optr, "%s requires value_kind iri", o)
		case !stdOps[o]:
			if oe, ok := ix.Lookup(o); !ok || oe.Kind != "operator" {
				fs.add(d, "N2", Error, CodeSigOperatorUnknown, optr, "operator %s is not an ODRL operator nor an operator of the profile", o)
			}
		}
	}
	if s.Kind() != "iri" {
		return
	}
	for i, v := range s.Values {
		vptr := ptr + "/values/" + itoa(i)
		if _, ok := ix.Expand(v); !ok {
			fs.add(d, "N2", Error, CodePrefixUndeclared, vptr, "prefix of %s is not declared", v)
			continue
		}
		if ve, ok := ix.Lookup(v); !ok || ve.Kind != "right_operand" {
			fs.add(d, "N2", Warning, CodeSigValueKind, vptr, "%s is not a right_operand of the profile", v)
		}
	}
}

// checkAdopted is N3 for one adopted term.
func checkAdopted(ix *Index, e *Entry, others []*Index, fs *Findings) {
	d := ix.Doc
	if strings.HasPrefix(e.CURIE, "odrl:") {
		kind, ok := odrlVocab[localName(e.CURIE)]
		switch {
		case !ok:
			fs.add(d, "N3", Error, CodeAdoptedUnknown, e.Ptr, "%s is not in the ODRL 2.2 vocabulary", e.CURIE)
		case kind != e.Kind:
			fs.add(d, "N3", Error, CodeAdoptedKindMismatch, e.Ptr, "%s is a %s in ODRL, not a %s", e.CURIE, kind, e.Kind)
		}
		return
	}
	for _, o := range others {
		if o == ix {
			continue
		}
		if src, ok := o.byIRI[e.IRI]; ok && src.Created != nil {
			if src.Kind != e.Kind {
				fs.add(d, "N3", Error, CodeAdoptedKindMismatch, e.Ptr, "%s is a %s in %s, not a %s", e.CURIE, src.Kind, o.P.Profile.ID, e.Kind)
			}
			return
		}
	}
	fs.add(d, "N3", Error, CodeAdoptedUnknown, e.Ptr, "%s is not defined by ODRL nor by any profile in the bundle", e.CURIE)
}
