// Unit tests for the RFC 8785 canonical form.
// Covers key ordering, string escaping and ECMAScript number rendering (REQ-HUB-010).
package canon

import "testing"

// mustT canonicalizes a JSON text or fails the test.
func mustT(t *testing.T, s string) string {
	t.Helper()
	b, err := Transform([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestKeysSortedByUTF16(t *testing.T) {
	got := mustT(t, `{"b":1,"a":2,"€":3,"😀":4,"aa":5}`)
	if want := `{"a":2,"aa":5,"b":1,"€":3,"😀":4}`; got != want {
		t.Fatalf("got %s", got)
	}
}

func TestWhitespaceRemoved(t *testing.T) {
	if got := mustT(t, "{ \"a\" : [ 1 , true , null ] }"); got != `{"a":[1,true,null]}` {
		t.Fatalf("got %s", got)
	}
}

func TestStringEscapes(t *testing.T) {
	if got := mustT(t, `"\u0001\n\"\\/<é"`); got != `"\u0001\n\"\\/<é"` {
		t.Fatalf("got %s", got)
	}
}

func TestNumbersAsECMAScript(t *testing.T) {
	cases := map[string]string{"1.0": "1", "-0": "0", "1e21": "1e+21", "1e20": "100000000000000000000", "0.000001": "0.000001", "1e-7": "1e-7", "123.456": "123.456", "418": "418"}
	for in, want := range cases {
		if got := mustT(t, in); got != want {
			t.Errorf("%s: got %s want %s", in, got, want)
		}
	}
}

func TestTrailingDataRejected(t *testing.T) {
	if _, err := Transform([]byte(`{} {}`)); err == nil {
		t.Fatal("trailing data accepted")
	}
}
