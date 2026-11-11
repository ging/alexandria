// Package domain checks vocabulary content by media type: JSON for the JSON family and a lexical Turtle check for text/turtle.
// The Turtle check is the minimal option coherent with §3.6 without a Turtle library (agent decision, REQ-HUB-026).
package domain

import (
	"encoding/json"
	"mime"
	"strings"
)

// Essence returns the media type without parameters, lower-cased.
func Essence(mt string) string {
	e, _, err := mime.ParseMediaType(mt)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(strings.Split(mt, ";")[0]))
	}
	return e
}

// isJSON reports whether a media type belongs to the JSON family checked by §3.6.
func isJSON(mt string) bool {
	switch Essence(mt) {
	case "application/json", "application/schema+json", "application/ld+json":
		return true
	}
	return false
}

// CompatibleMediaType reports whether a served Content-Type fits the declared media type (agent decision: equal essence, or JSON vs +json).
func CompatibleMediaType(declared, served string) bool {
	d, s := Essence(declared), Essence(served)
	if d == s {
		return true
	}
	return (s == "application/json" && strings.HasSuffix(d, "+json")) || (d == "application/json" && strings.HasSuffix(s, "+json"))
}

// Parseable reports whether b is valid for the media type; unchecked types are always parseable.
func Parseable(mt string, b []byte) bool {
	switch {
	case isJSON(mt):
		return json.Valid(b)
	case Essence(mt) == "text/turtle":
		return TurtleLexOK(string(b))
	}
	return true
}

// TurtleLexOK checks that IRIs, strings and brackets are closed and that a non-empty document ends its last statement.
func TurtleLexOK(s string) bool {
	depth, last := 0, byte(0)
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '#':
			for i < len(s) && s[i] != '\n' {
				i++
			}
			continue
		case '<':
			j := strings.IndexByte(s[i+1:], '>')
			if j < 0 || strings.ContainsAny(s[i+1:i+1+j], " \n\"{}|^`") {
				return false
			}
			i += j + 1
		case '"', '\'':
			end := skipString(s, i)
			if end < 0 {
				return false
			}
			i = end
		case '[', '(':
			depth++
		case ']', ')':
			depth--
			if depth < 0 {
				return false
			}
		case '{', '}':
			return false
		}
		if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
			last = c
		}
	}
	return depth == 0 && (last == 0 || last == '.')
}

// skipString returns the index of the closing quote of the literal starting at i, or -1.
func skipString(s string, i int) int {
	q := s[i]
	long := strings.HasPrefix(s[i:], strings.Repeat(string(q), 3))
	if long {
		end := strings.Index(s[i+3:], strings.Repeat(string(q), 3))
		if end < 0 {
			return -1
		}
		return i + 3 + end + 2
	}
	for j := i + 1; j < len(s); j++ {
		switch s[j] {
		case '\\':
			j++
		case '\n':
			return -1
		case q:
			return j
		}
	}
	return -1
}
