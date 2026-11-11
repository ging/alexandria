// Package domain evaluates JSON Pointers (RFC 6901) and the exists, equals and one_of body conditions of a rule.
// Values are compared with JSON equality on decoded values (§4.3.2, REQ-CLI-030).
package domain

import (
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
)

// Decode parses a JSON body; an empty body decodes to null (agent decision for body-less requests).
func Decode(body []byte) (any, bool) {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil, true
	}
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, false
	}
	return v, true
}

// Pointer resolves an RFC 6901 JSON Pointer against a decoded document.
func Pointer(doc any, ptr string) (any, bool) {
	if ptr == "" {
		return doc, true
	}
	if !strings.HasPrefix(ptr, "/") {
		return nil, false
	}
	cur := doc
	for _, tok := range strings.Split(ptr[1:], "/") {
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		switch n := cur.(type) {
		case map[string]any:
			v, ok := n[tok]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			i, err := strconv.Atoi(tok)
			if err != nil || i < 0 || i >= len(n) || (len(tok) > 1 && tok[0] == '0') {
				return nil, false
			}
			cur = n[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

// Condition is one entry of match.body[]: exactly one of Exists, Equals and OneOf.
type Condition struct {
	Pointer string            `json:"pointer"`
	Exists  *bool             `json:"exists,omitempty"`
	Equals  json.RawMessage   `json:"equals,omitempty"`
	OneOf   []json.RawMessage `json:"one_of,omitempty"`
}

// Holds evaluates the condition on a decoded body.
func (c Condition) Holds(doc any) bool {
	v, ok := Pointer(doc, c.Pointer)
	switch {
	case c.Exists != nil:
		return ok == *c.Exists
	case c.Equals != nil:
		return ok && jsonEqual(v, c.Equals)
	default:
		if !ok {
			return false
		}
		for _, o := range c.OneOf {
			if jsonEqual(v, o) {
				return true
			}
		}
		return false
	}
}

// jsonEqual compares a decoded value with a raw JSON value.
func jsonEqual(v any, raw json.RawMessage) bool {
	var o any
	if err := json.Unmarshal(raw, &o); err != nil {
		return false
	}
	return reflect.DeepEqual(v, o)
}

// Field returns the first string member of the root object among names.
func Field(doc any, names ...string) (string, bool) {
	m, ok := doc.(map[string]any)
	if !ok {
		return "", false
	}
	for _, n := range names {
		if s, ok := m[n].(string); ok {
			return s, true
		}
	}
	return "", false
}
