// Tests the §3.5 authorization table cell by cell and the slug and vocabulary name patterns.
// Fixtures: an admin, a plain principal and a steward principal (T-HUB-017, T-HUB-025, T-HUB-026).
package domain

import "testing"

// plain, admin are the principals of the matrix.
var (
	plain = &Principal{Iss: "https://idp", Sub: "u"}
	admin = &Principal{Iss: "https://idp", Sub: "a", IsAdmin: true}
)

func TestT_HUB_017_AuthzMatrix(t *testing.T) {
	type cell struct {
		op      Op
		p       *Principal
		steward bool
		want    Decision
	}
	cells := []cell{
		{OpPublic, nil, false, Allow},
		{OpPublic, plain, false, Allow},
		{OpPublic, plain, true, Allow},
		{OpPublic, admin, false, Allow},
		{OpAdmin, nil, false, Unauthenticated},
		{OpAdmin, plain, false, Forbidden},
		{OpAdmin, plain, true, Forbidden},
		{OpAdmin, admin, false, Allow},
		{OpListStewards, nil, false, Unauthenticated},
		{OpListStewards, plain, false, Forbidden},
		{OpListStewards, plain, true, Allow},
		{OpListStewards, admin, false, Allow},
		{OpNamespace, nil, false, Unauthenticated},
		{OpNamespace, plain, false, Forbidden},
		{OpNamespace, plain, true, Allow},
		{OpNamespace, admin, false, Forbidden},
		{OpNamespace, admin, true, Allow},
	}
	for i, c := range cells {
		if got := Decide(c.op, c.p, c.steward); got != c.want {
			t.Errorf("cell %d: got %v want %v", i, got, c.want)
		}
	}
}

func TestT_HUB_025_SlugPattern(t *testing.T) {
	for s, ok := range map[string]bool{"demo": true, "a1-b": true, "Demo_1": false, "_vhub": false, "a": false, "-a": false} {
		if ValidSlug(s) != ok {
			t.Errorf("%q: got %v", s, !ok)
		}
	}
}

func TestT_HUB_026_VocabNamePattern(t *testing.T) {
	for s, ok := range map[string]bool{"dcat-ap": true, "a.b_c": true, "Mi Vocab": false, "": false} {
		if ValidVocabName(s) != ok {
			t.Errorf("%q: got %v", s, !ok)
		}
	}
}
