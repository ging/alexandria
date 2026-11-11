// Construction guarantees of F8 §8.6: the hot path holds no hub client nor DID resolver, and Explain holds no effectful port.
// Checked by reflection over the struct fields, embedded ones included (Part 4 M6, REQ-SYS-002, REQ-CLI-032).
package archtest

import (
	"reflect"
	"testing"

	"github.com/ging/alexandria/internal/client/app"
)

// fieldTypes returns every field type of a struct, descending into embedded structs.
func fieldTypes(t reflect.Type) []reflect.Type {
	var out []reflect.Type
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			out = append(out, fieldTypes(f.Type)...)
			continue
		}
		out = append(out, f.Type)
	}
	return out
}

// holds reports whether a struct has a field of interface type iface.
func holds(s reflect.Type, iface reflect.Type) bool {
	for _, ft := range fieldTypes(s) {
		if ft == iface || (iface.Kind() == reflect.Interface && ft.Kind() == reflect.Interface && ft.Implements(iface) && iface.NumMethod() > 0) {
			return true
		}
	}
	return false
}

// typ returns the reflected type of an interface pointer.
func typ[T any]() reflect.Type { return reflect.TypeOf((*T)(nil)).Elem() }

func TestEdgeHasNoHubNorResolver(t *testing.T) {
	for _, iface := range []reflect.Type{typ[app.HubAPI](), typ[app.DIDResolver]()} {
		for _, s := range []reflect.Type{typ[app.Edge](), typ[app.Explainer]()} {
			if holds(s, iface) {
				t.Errorf("%s holds a %s", s.Name(), iface.Name())
			}
		}
	}
}

func TestExplainHasNoEffects(t *testing.T) {
	for _, iface := range []reflect.Type{typ[app.Upstream](), typ[app.AgreementRepo](), typ[app.EventSink]()} {
		if holds(typ[app.Explainer](), iface) {
			t.Errorf("Explainer holds a %s", iface.Name())
		}
	}
	if holds(typ[app.Explainer](), reflect.TypeOf(&app.Agreements{})) {
		t.Error("Explainer holds the agreement capture")
	}
}
