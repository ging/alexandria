// Package schemas embeds the six Appendix A schemas and the A.5 JSON-LD context, with a validator that never touches the network.
// Used to check every document the binaries emit or read (REQ-DOC-001, REQ-DOC-002, REQ-DOC-003, REQ-DOC-004, REQ-DOC-005, REQ-DOC-006, REQ-HUB-030, REQ-HUB-031).
package schemas

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed *.schema.json context.jsonld
var files embed.FS

// Kind names one of the six normative schemas.
type Kind string

// Normative schema files of Appendix A.
const (
	Versions Kind = "versions.schema.json"
	Version  Kind = "version.schema.json"
	Config   Kind = "config.schema.json"
	Verdict  Kind = "verdict-event.schema.json"
	Manifest Kind = "manifest.schema.json"
	Rules    Kind = "rules.schema.json"
)

// All lists the six normative schemas.
var All = []Kind{Versions, Version, Config, Verdict, Manifest, Rules}

// Context returns the Appendix A.5 JSON-LD context byte for byte.
func Context() []byte { b, _ := files.ReadFile("context.jsonld"); return b }

// Schema is a compiled JSON Schema 2020-12.
type Schema struct{ s *jsonschema.Schema }

var compiled = map[Kind]*Schema{}

func init() {
	for _, k := range All {
		b, err := files.ReadFile(string(k))
		if err != nil {
			panic(err)
		}
		s, err := Compile(b)
		if err != nil {
			panic(fmt.Sprintf("schemas: %s: %v", k, err))
		}
		compiled[k] = s
	}
}

// Raw returns the embedded bytes of a normative schema.
func Raw(k Kind) []byte { b, _ := files.ReadFile(string(k)); return b }

// Validate checks a JSON document against a normative schema.
func Validate(k Kind, doc []byte) error { return compiled[k].Validate(doc) }

// Compile compiles a schema document; remote references are never resolved.
func Compile(schema []byte) (*Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.UseLoader(noLoader{})
	c.UseRegexpEngine(compileRegexp)
	if err := c.AddResource("urn:vhub:schema", doc); err != nil {
		return nil, err
	}
	s, err := c.Compile("urn:vhub:schema")
	if err != nil {
		return nil, err
	}
	return &Schema{s}, nil
}

// Validate checks a JSON document.
func (s *Schema) Validate(doc []byte) error {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(doc))
	if err != nil {
		return err
	}
	return s.s.Validate(inst)
}

// noLoader refuses to load any external resource (D6: nothing resolves against the network).
type noLoader struct{}

// Load always fails.
func (noLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("schemas: external reference %s is not allowed", url)
}

// lookahead matches a pattern made of an anchored prefix, one positive lookahead and a remainder.
var lookahead = regexp.MustCompile(`^(\^[^()]*)\(\?=([^()]*)\)(.*)$`)

// compileRegexp uses RE2 and emulates the single leading lookahead of trust.did_ttl, which RE2 lacks.
func compileRegexp(p string) (jsonschema.Regexp, error) {
	if re, err := regexp.Compile(p); err == nil {
		return re, nil
	}
	m := lookahead.FindStringSubmatch(p)
	if m == nil {
		return nil, fmt.Errorf("schemas: unsupported pattern %q", p)
	}
	prefix, err := regexp.Compile(m[1])
	if err != nil {
		return nil, err
	}
	ahead, err := regexp.Compile("^(?:" + m[2] + ")")
	if err != nil {
		return nil, err
	}
	whole, err := regexp.Compile(m[1] + m[3])
	if err != nil {
		return nil, err
	}
	return lookaheadRegexp{p, prefix, ahead, whole}, nil
}

// lookaheadRegexp evaluates prefix(?=ahead)rest as prefix·rest plus an anchored check after the prefix.
type lookaheadRegexp struct {
	src                  string
	prefix, ahead, whole *regexp.Regexp
}

// MatchString reports whether s matches the emulated pattern.
func (l lookaheadRegexp) MatchString(s string) bool {
	loc := l.prefix.FindStringIndex(s)
	if loc == nil || !l.whole.MatchString(s) {
		return false
	}
	return l.ahead.MatchString(s[loc[1]:])
}

// String returns the original pattern.
func (l lookaheadRegexp) String() string { return strings.Clone(l.src) }

// Leaf is one validation error with its instance location.
type Leaf struct {
	Path    string
	Message string
}

// Errors validates doc and returns one leaf per basic-output error, or nil when valid.
func (s *Schema) Errors(doc []byte) []Leaf {
	err := s.Validate(doc)
	if err == nil {
		return nil
	}
	var ve *jsonschema.ValidationError
	if !errors.As(err, &ve) {
		return []Leaf{{Message: err.Error()}}
	}
	var out []Leaf
	for _, u := range ve.BasicOutput().Errors {
		if u.Error != nil {
			out = append(out, Leaf{Path: u.InstanceLocation, Message: u.Error.String()})
		}
	}
	if len(out) == 0 {
		out = append(out, Leaf{Message: err.Error()})
	}
	return out
}
