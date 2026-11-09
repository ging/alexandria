// Lightweight Turtle well-formedness check for N5: balanced brackets and parentheses, closed strings and IRIs, final dot.
// Not a full parser: a semantic check needs an RDF library, which this module does not depend on (§12 N5).
package dsl

import "fmt"

// checkTurtle scans a Turtle document and reports the first structural error.
func checkTurtle(b []byte) error {
	var stack []byte
	line := 1
	lastSignificant := byte(0)
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch c {
		case '\n':
			line++
			continue
		case '#':
			for i < len(b) && b[i] != '\n' {
				i++
			}
			line++
			continue
		case '"':
			j := i + 1
			for ; j < len(b) && b[j] != '"'; j++ {
				if b[j] == '\\' {
					j++
				}
				if j < len(b) && b[j] == '\n' {
					return fmt.Errorf("line %d: unterminated string", line)
				}
			}
			if j >= len(b) {
				return fmt.Errorf("line %d: unterminated string", line)
			}
			i, lastSignificant = j, '"'
			continue
		case '<':
			j := i + 1
			for ; j < len(b) && b[j] != '>'; j++ {
				if b[j] == ' ' || b[j] == '\n' || b[j] == '"' {
					return fmt.Errorf("line %d: unterminated IRI", line)
				}
			}
			if j >= len(b) {
				return fmt.Errorf("line %d: unterminated IRI", line)
			}
			i, lastSignificant = j, '>'
			continue
		case '[', '(':
			stack = append(stack, c)
		case ']', ')':
			want := byte('[')
			if c == ')' {
				want = '('
			}
			if len(stack) == 0 || stack[len(stack)-1] != want {
				return fmt.Errorf("line %d: unbalanced %q", line, c)
			}
			stack = stack[:len(stack)-1]
		}
		if c != ' ' && c != '\t' && c != '\r' {
			lastSignificant = c
		}
	}
	if len(stack) > 0 {
		return fmt.Errorf("unclosed %q", stack[len(stack)-1])
	}
	if lastSignificant != 0 && lastSignificant != '.' {
		return fmt.Errorf("document does not end with '.'")
	}
	return nil
}
