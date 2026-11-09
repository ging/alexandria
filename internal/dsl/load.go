// Package dsl is the loader of YAML streams restricted to the JSON data model, and level N1 against the normative schema.
// Rejects anchors, aliases and explicit tags, and remembers the line of every JSON pointer for findings (§6, §12 N1).
package dsl

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/schemas"
	"gopkg.in/yaml.v3"
)

//go:embed schema.json
var schemaJSON []byte

// SchemaJSON returns the normative schema used for N1.
func SchemaJSON() []byte { return schemaJSON }

var compiledSchema = mustCompile(schemaJSON)

// mustCompile compiles the embedded schema once.
func mustCompile(b []byte) *schemas.Schema {
	s, err := schemas.Compile(b)
	if err != nil {
		panic(fmt.Sprintf("dsl: schema: %v", err))
	}
	return s
}

// Source is one named input file.
type Source struct {
	Name string
	Data []byte
}

// Load parses every YAML document of every source and runs N1.
func Load(srcs ...Source) ([]*Document, Findings) {
	var docs []*Document
	var fs Findings
	for _, src := range srcs {
		dec := yaml.NewDecoder(bytes.NewReader(src.Data))
		for i := 1; ; i++ {
			var n yaml.Node
			err := dec.Decode(&n)
			if errors.Is(err, io.EOF) {
				break
			}
			d := &Document{Name: src.Name, Index: i, Lines: map[string]int{}}
			if err != nil {
				fs.add(d, "N1", Error, CodeYAML, "", "%v", err)
				break
			}
			v, err := toJSON(&n, "", d.Lines)
			if err != nil {
				fs.add(d, "N1", Error, CodeYAML, "", "%v", err)
				continue
			}
			d.JSON, _ = json.Marshal(v)
			d.Source = src.Data
			if f := schemaFindings(d); len(f) > 0 {
				fs = append(fs, f...)
				continue
			}
			if err := decodeTyped(d); err != nil {
				fs.add(d, "N1", Error, CodeSchema, "", "%v", err)
				continue
			}
			docs = append(docs, d)
		}
	}
	return docs, fs
}

// schemaFindings validates a document against the normative schema.
func schemaFindings(d *Document) Findings {
	var fs Findings
	for _, l := range compiledSchema.Errors(d.JSON) {
		fs.add(d, "N1", Error, CodeSchema, l.Path, "%s", l.Message)
	}
	return fs
}

// decodeTyped fills the typed view of a schema-valid document.
func decodeTyped(d *Document) error {
	var head struct{ Kind string }
	if err := json.Unmarshal(d.JSON, &head); err != nil {
		return err
	}
	d.Kind = head.Kind
	switch head.Kind {
	case KindOdrlProfile:
		d.Odrl = &OdrlProfile{}
		return json.Unmarshal(d.JSON, d.Odrl)
	case KindSpaceProfile:
		d.Space = &SpaceProfile{}
		return json.Unmarshal(d.JSON, d.Space)
	}
	return fmt.Errorf("unsupported kind %q", head.Kind)
}

// toJSON converts a YAML node into a JSON-model value, recording lines by JSON pointer.
func toJSON(n *yaml.Node, ptr string, lines map[string]int) (any, error) {
	if n.Kind == yaml.DocumentNode {
		if len(n.Content) == 0 {
			return nil, errors.New("empty document")
		}
		return toJSON(n.Content[0], ptr, lines)
	}
	lines[ptr] = n.Line
	switch {
	case n.Anchor != "":
		return nil, fmt.Errorf("line %d: anchors are not allowed (§6)", n.Line)
	case n.Kind == yaml.AliasNode:
		return nil, fmt.Errorf("line %d: aliases are not allowed (§6)", n.Line)
	case n.Style&yaml.TaggedStyle != 0:
		return nil, fmt.Errorf("line %d: explicit tags are not allowed (§6)", n.Line)
	}
	switch n.Kind {
	case yaml.MappingNode:
		out := map[string]any{}
		for i := 0; i+1 < len(n.Content); i += 2 {
			k := n.Content[i]
			if k.Kind != yaml.ScalarNode || (k.Tag != "!!str" && k.Tag != "!!int" && k.Tag != "!!bool") {
				return nil, fmt.Errorf("line %d: mapping keys must be strings", k.Line)
			}
			if _, dup := out[k.Value]; dup {
				return nil, fmt.Errorf("line %d: duplicate key %q", k.Line, k.Value)
			}
			v, err := toJSON(n.Content[i+1], ptr+"/"+escapePtr(k.Value), lines)
			if err != nil {
				return nil, err
			}
			out[k.Value] = v
		}
		return out, nil
	case yaml.SequenceNode:
		out := make([]any, 0, len(n.Content))
		for i, c := range n.Content {
			v, err := toJSON(c, ptr+"/"+strconv.Itoa(i), lines)
			if err != nil {
				return nil, err
			}
			out = append(out, v)
		}
		return out, nil
	}
	return scalar(n)
}

// scalar converts a YAML scalar; timestamps stay as their text, as in the JSON model.
func scalar(n *yaml.Node) (any, error) {
	switch n.Tag {
	case "!!null":
		return nil, nil
	case "!!bool":
		return n.Value == "true", nil
	case "!!int":
		i, err := strconv.ParseInt(n.Value, 0, 64)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", n.Line, err)
		}
		return i, nil
	case "!!float":
		f, err := strconv.ParseFloat(n.Value, 64)
		if err != nil || math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("line %d: %q is not a JSON number", n.Line, n.Value)
		}
		return f, nil
	}
	return n.Value, nil
}

// escapePtr escapes a key for a JSON pointer (RFC 6901).
func escapePtr(k string) string { return strings.NewReplacer("~", "~0", "/", "~1").Replace(k) }
