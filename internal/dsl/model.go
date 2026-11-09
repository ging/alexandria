// Package dsl defines the document model of the eunomia.profiles/v1 language: OdrlProfile and SpaceProfile (ConformanceSuite is out of scope).
// Typed view of a YAML document after it has passed the normative schema (Parte II §7–§11).
package dsl

import (
	"encoding/json"
	"fmt"
)

// Language identifier and document kinds of §7.
const (
	Language         = "eunomia.profiles/v1"
	KindOdrlProfile  = "OdrlProfile"
	KindSpaceProfile = "SpaceProfile"
)

// Reserved prefixes of §6 and the IRIs they stand for.
var Reserved = map[string]string{
	"odrl": "http://www.w3.org/ns/odrl/2/",
	"xsd":  "http://www.w3.org/2001/XMLSchema#",
	"rdf":  "http://www.w3.org/1999/02/22-rdf-syntax-ns#",
	"rdfs": "http://www.w3.org/2000/01/rdf-schema#",
	"owl":  "http://www.w3.org/2002/07/owl#",
	"skos": "http://www.w3.org/2004/02/skos/core#",
	"dct":  "http://purl.org/dc/terms/",
	"sig":  "https://w3id.org/eunomia/ns/signature#",
	"eu":   "https://w3id.org/eunomia/ns/",
}

// LangMap maps a BCP 47 tag to text; it always contains "en".
type LangMap map[string]string

// Signature is the algebra of a left operand (§9).
type Signature struct {
	Operators      []string `json:"operators"`
	Datatype       string   `json:"datatype,omitempty"`
	ValueKind      string   `json:"value_kind,omitempty"`
	Values         []string `json:"values,omitempty"`
	Scheme         string   `json:"scheme,omitempty"`
	AllowReference bool     `json:"allow_reference,omitempty"`
}

// Enforcement describes how the PDP evaluates a left operand (§10).
type Enforcement struct {
	CedarPath  string `json:"cedar_path"`
	PDPLevel   string `json:"pdp_level,omitempty"`
	EvalTiming string `json:"eval_timing,omitempty"`
}

// Adopted is one adopted term, written as a CURIE or as {term, signature, enforcement} (§8.4).
type Adopted struct {
	Term        string       `json:"term"`
	Signature   *Signature   `json:"signature,omitempty"`
	Enforcement *Enforcement `json:"enforcement,omitempty"`
}

// UnmarshalJSON accepts the short CURIE form and the long object form.
func (a *Adopted) UnmarshalJSON(b []byte) error {
	var s string
	if json.Unmarshal(b, &s) == nil {
		*a = Adopted{Term: s}
		return nil
	}
	type long Adopted
	var l long
	if err := json.Unmarshal(b, &l); err != nil {
		return err
	}
	*a = Adopted(l)
	return nil
}

// Term is one term defined by the profile (§8.5).
type Term struct {
	ID          string       `json:"id"`
	Kind        string       `json:"kind"`
	Label       LangMap      `json:"label"`
	Definition  LangMap      `json:"definition"`
	Note        LangMap      `json:"note,omitempty"`
	IncludedIn  string       `json:"included_in,omitempty"`
	SubclassOf  string       `json:"subclass_of,omitempty"`
	Signature   *Signature   `json:"signature,omitempty"`
	Enforcement *Enforcement `json:"enforcement,omitempty"`
	Status      string       `json:"status,omitempty"`
	ReplacedBy  string       `json:"replaced_by,omitempty"`
	ExtraTurtle string       `json:"extra_turtle,omitempty"`
}

// State returns the lifecycle status with its active default (§25).
func (t Term) State() string {
	if t.Status == "" {
		return "active"
	}
	return t.Status
}

// ProfileMeta is the profile section (§8.2).
type ProfileMeta struct {
	ID          string  `json:"id"`
	Version     string  `json:"version"`
	Title       LangMap `json:"title"`
	Description LangMap `json:"description,omitempty"`
	Publisher   string  `json:"publisher"`
	License     string  `json:"license"`
	BaseURL     string  `json:"base_url"`
}

// Rules is the closure policy of the vocabulary (§8.6).
type Rules struct {
	Actions      string `json:"actions,omitempty"`
	LeftOperands string `json:"left_operands,omitempty"`
}

// ActionsClosed reports whether the action list is closed (the default).
func (r Rules) ActionsClosed() bool { return r.Actions != "open" }

// LeftOperandsClosed reports whether the left-operand list is closed (the default).
func (r Rules) LeftOperandsClosed() bool { return r.LeftOperands != "open" }

// Categories of adopted terms in §8.4, in emission order.
var Categories = []string{"actions", "left_operands", "operators", "right_operands", "party_functions", "asset_relations"}

// OdrlProfile is kind: OdrlProfile.
type OdrlProfile struct {
	DSL        string               `json:"dsl"`
	Kind       string               `json:"kind"`
	Profile    ProfileMeta          `json:"profile"`
	Namespaces map[string]string    `json:"namespaces"`
	Adopted    map[string][]Adopted `json:"adopted,omitempty"`
	Terms      []Term               `json:"terms,omitempty"`
	Rules      Rules                `json:"rules,omitempty"`
}

// SpaceMeta is the space section (§11.1).
type SpaceMeta struct {
	ID          string  `json:"id"`
	Version     string  `json:"version"`
	Title       LangMap `json:"title"`
	Description LangMap `json:"description,omitempty"`
	Operator    string  `json:"operator"`
	BaseURL     string  `json:"base_url"`
}

// Conformance is one (role, applies_to) pair of a binding.
type Conformance struct {
	Role      string `json:"role"`
	AppliesTo string `json:"applies_to"`
}

// Binding fixes the profile that governs a slot (§11.3).
type Binding struct {
	Slot        string        `json:"slot"`
	Profile     string        `json:"profile"`
	Source      string        `json:"source"`
	Version     string        `json:"version"`
	Obligation  string        `json:"obligation"`
	Via         []string      `json:"via,omitempty"`
	Conformance []Conformance `json:"conformance,omitempty"`
}

// SpaceProfile is kind: SpaceProfile; Lifecycle is kept raw and read with core/lifecycle.
type SpaceProfile struct {
	DSL       string          `json:"dsl"`
	Kind      string          `json:"kind"`
	Space     SpaceMeta       `json:"space"`
	Lifecycle json.RawMessage `json:"lifecycle,omitempty"`
	Bindings  []Binding       `json:"bindings"`
}

// Document is one loaded YAML document with its JSON form and source positions.
type Document struct {
	Name   string
	Index  int
	Kind   string
	JSON   []byte
	Lines  map[string]int
	Odrl   *OdrlProfile
	Space  *SpaceProfile
	Source []byte
}

// Label names a document for findings, such as colors.profile.yaml#1.
func (d *Document) Label() string { return fmt.Sprintf("%s#%d", d.Name, d.Index) }

// Line returns the source line of a JSON pointer, falling back to its closest ancestor.
func (d *Document) Line(ptr string) int {
	for p := ptr; ; {
		if l, ok := d.Lines[p]; ok {
			return l
		}
		i := lastSlash(p)
		if i < 0 {
			return 0
		}
		p = p[:i]
	}
}

// lastSlash returns the index of the last '/' of a pointer, or -1.
func lastSlash(p string) int {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' {
			return i
		}
	}
	return -1
}
