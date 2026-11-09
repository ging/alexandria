// Tests of offline expansion: declared contexts resolve, any other URL fails as NotInPackage, nothing reaches the network.
// Covers the loader behaviour that D-PVP-1 and REQ-CLI-010 rely on.
package jsonld

import "testing"

// ctxURL is a context URL served by the test loader.
const ctxURL = "https://example.com/odrl/profile/colors/context.jsonld"

// loader returns a loader with the colors context.
func loader(t *testing.T) *Loader {
	t.Helper()
	l := NewLoader()
	if err := l.Add(ctxURL, []byte(`{"@context":{"col":"https://example.com/ns/colors#","putColor":"col:putColor"}}`)); err != nil {
		t.Fatal(err)
	}
	return l
}

func TestExpandsWithDeclaredContext(t *testing.T) {
	out, err := Expand([]byte(`{"@context":"`+ctxURL+`","@id":"urn:x","@type":"putColor"}`), loader(t))
	if err != nil {
		t.Fatal(err)
	}
	types := out[0].(map[string]any)["@type"].([]any)
	if types[0] != "https://example.com/ns/colors#putColor" {
		t.Fatalf("got %v", types)
	}
}

func TestUndeclaredContextIsNotInPackage(t *testing.T) {
	_, err := Expand([]byte(`{"@context":["`+ctxURL+`","https://evil.example/ctx.jsonld"],"@id":"urn:x"}`), loader(t))
	if url, ok := MissingContext(err); !ok || url != "https://evil.example/ctx.jsonld" {
		t.Fatalf("got %v", err)
	}
}

func TestNestedScopedContextIsAlsoChecked(t *testing.T) {
	doc := `{"@context":"` + ctxURL + `","@id":"urn:x","https://example.com/p":{"@context":"https://nested.example/ctx","@id":"urn:y"}}`
	if url, ok := MissingContext(func() error { _, err := Expand([]byte(doc), loader(t)); return err }()); !ok || url != "https://nested.example/ctx" {
		t.Fatalf("nested context not caught: %s %v", url, ok)
	}
}
