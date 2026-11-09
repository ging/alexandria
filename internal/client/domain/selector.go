// Package domain provides a minimal JSONPath (RFC 9535) subset for body.select: $, .name, ['name'] and [index] segments.
// A body that is not JSON, or a path that does not select exactly one node, is MESSAGE_UNPARSEABLE (§4.3, §4.7, REQ-CLI-011).
package domain

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
)

var segmentRe = regexp.MustCompile(`^(?:\.([A-Za-z_@][A-Za-z0-9_@:-]*)|\['([^']*)'\]|\["([^"]*)"\]|\[([0-9]+)\])`)

// SelectBody returns the JSON of the single node selected by path, or false.
func SelectBody(body []byte, path string) ([]byte, bool) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, false
	}
	if !strings.HasPrefix(path, "$") {
		return nil, false
	}
	rest := path[1:]
	for rest != "" {
		m := segmentRe.FindStringSubmatch(rest)
		if m == nil {
			return nil, false
		}
		rest = rest[len(m[0]):]
		switch {
		case m[4] != "":
			arr, ok := v.([]any)
			i, _ := strconv.Atoi(m[4])
			if !ok || i >= len(arr) {
				return nil, false
			}
			v = arr[i]
		default:
			obj, ok := v.(map[string]any)
			name := m[1] + m[2] + m[3]
			if !ok {
				return nil, false
			}
			if v, ok = obj[name]; !ok {
				return nil, false
			}
		}
	}
	out, err := json.Marshal(v)
	return out, err == nil
}

// Field returns a top-level string field of a JSON object, trying each name in turn.
func Field(body []byte, names ...string) string {
	var obj map[string]any
	if json.Unmarshal(body, &obj) != nil {
		return ""
	}
	for _, n := range names {
		if s, ok := obj[n].(string); ok {
			return s
		}
	}
	return ""
}
