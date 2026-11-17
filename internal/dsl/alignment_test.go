// Tests that shapes.ttl and schema.json state the same rules: declared profile, mandatory action, operands, lexical forms and messages.
// The equivalence over 27 policies was checked with pySHACL (tutorial/manual/examples/shacl); these tests pin the emitted text and the schema verdicts.
package dsl

import (
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/schemas"
)

// colorsID is the profile id of the colors fixture.
const colorsID = "https://example.com/odrl/profile/colors"

// colorsShapes returns shapes.ttl of the colors fixture after the given replacements.
func colorsShapes(t *testing.T, repl ...string) string {
	t.Helper()
	return string(artifact(t, Compile(Options{}, colors(t, repl...)), "profiles/odrl/colors/0.1.0/shapes.ttl"))
}

// colorsSchema compiles schema.json of the colors fixture after the given replacements.
func colorsSchema(t *testing.T, repl ...string) *schemas.Schema {
	t.Helper()
	s, err := schemas.Compile(artifact(t, Compile(Options{}, colors(t, repl...)), "profiles/odrl/colors/0.1.0/schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// dated builds a dateTime constraint with a typed right operand.
func dated(value string) string {
	return `{"action":"use","constraint":[{"leftOperand":"dateTime","operator":"lt","rightOperand":{"@value":"` + value + `","@type":"xsd:dateTime"}}]}`
}

func TestShapesRequireDeclaredProfile(t *testing.T) {
	sh := colorsShapes(t)
	want := "sh:qualifiedValueShape [ sh:in ( <" + colorsID + "> <" + colorsID + "/0.1.0> ) ] ;\n    sh:qualifiedMinCount 1"
	if !strings.Contains(sh, "gen:ProfileShape a sh:NodeShape") || !strings.Contains(sh, want) {
		t.Fatalf("shapes.ttl does not require odrl:profile:\n%s", sh)
	}
}

func TestShapeMessagesLiveInThePropertyShape(t *testing.T) {
	sh := colorsShapes(t)
	for _, want := range []string{
		"sh:nodeKind sh:IRI ;\n    sh:message \"Action not allowed by profile colors.\"",
		"sh:severity sh:Warning ;\n    sh:message \"ODRL Core actions are ignored by profile colors.\"",
		"sh:maxCount 1 ;\n    sh:message \"LeftOperand not defined by profile colors.\"",
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("shapes.ttl lacks %q", want)
		}
	}
}

func TestCoreActionWarningTargetsOnlyCoreActions(t *testing.T) {
	if sh := colorsShapes(t); !strings.Contains(sh, "sh:not [ sh:in ( odrl:use odrl:transfer ) ]") {
		t.Fatalf("gen:ActionCoreShape does not single out the Core actions:\n%s", sh)
	}
}

func TestShapesRequireOperatorAndOperand(t *testing.T) {
	sh := colorsShapes(t)
	for _, want := range []string{
		"gen:ConstraintShape a sh:NodeShape",
		"sh:path odrl:operator ; sh:minCount 1",
		"[ sh:path odrl:rightOperand ; sh:minCount 1 ]\n    [ sh:path odrl:rightOperandReference ; sh:minCount 1 ]",
	} {
		if !strings.Contains(sh, want) {
			t.Errorf("shapes.ttl lacks %q", want)
		}
	}
}

func TestSchemaRequiresActionWhenActionsAreClosed(t *testing.T) {
	if colorsSchema(t).Validate(offer(colorsID, `{"constraint":[]}`)) == nil {
		t.Fatal("a rule without action passed a profile with closed actions")
	}
}

func TestSchemaAllowsRuleWithoutActionWhenActionsAreOpen(t *testing.T) {
	s := colorsSchema(t, "actions: closed", "actions: open")
	if err := s.Validate(offer(colorsID, `{"constraint":[]}`)); err != nil {
		t.Fatalf("a rule without action failed a profile with open actions: %v", err)
	}
}

func TestSchemaRejectsIllFormedTypedValue(t *testing.T) {
	if colorsSchema(t).Validate(offer(colorsID, dated("tomorrow"))) == nil {
		t.Fatal("an xsd:dateTime that is not a date passed")
	}
}

func TestSchemaAcceptsWellFormedTypedValue(t *testing.T) {
	if err := colorsSchema(t).Validate(offer(colorsID, dated("2026-01-01T00:00:00+01:00"))); err != nil {
		t.Fatalf("a well-formed xsd:dateTime failed: %v", err)
	}
}

func TestN2MandatoryGateCoveredOnlyByShaclIsWarned(t *testing.T) {
	r := Compile(Options{}, colors(t), space("1.0.0", "^0.1", ""))
	expectErrors(t, r)
	if got := strings.Join(codes(r, Warning), ","); !strings.Contains(got, CodeGateShaclOnly) {
		t.Fatalf("warnings %q lack %s", got, CodeGateShaclOnly)
	}
}

func TestN2GateWithSchemaAndShaclIsNotWarned(t *testing.T) {
	sp := space("1.0.0", "^0.1", "")
	sp.Data = []byte(strings.Replace(string(sp.Data), "applies_to: dsp_message", "applies_to: policy_intake", 1))
	r := Compile(Options{}, colors(t), sp)
	expectErrors(t, r)
	if got := strings.Join(codes(r, Warning), ","); strings.Contains(got, CodeGateShaclOnly) {
		t.Fatalf("unexpected %s in %q", CodeGateShaclOnly, got)
	}
}
