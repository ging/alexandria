// Unit tests for SemVer parsing and precedence.
// Covers the ordering rules of SemVer 2.0.0 §11.
package semver

import "testing"

func TestParseRejectsInvalid(t *testing.T) {
	for _, s := range []string{"1.0", "01.0.0", "1.0.0-", "v1.0.0"} {
		if _, err := Parse(s); err == nil {
			t.Errorf("%s accepted", s)
		}
	}
}

func TestPrecedenceChain(t *testing.T) {
	chain := []string{"1.0.0-alpha", "1.0.0-alpha.1", "1.0.0-alpha.beta", "1.0.0-beta", "1.0.0-beta.2", "1.0.0-beta.11", "1.0.0-rc.1", "1.0.0", "1.0.1", "1.1.0", "2.0.0"}
	for i := 0; i+1 < len(chain); i++ {
		if !Less(MustParse(chain[i]), MustParse(chain[i+1])) {
			t.Errorf("%s should precede %s", chain[i], chain[i+1])
		}
	}
}

func TestBuildMetadataIgnored(t *testing.T) {
	if Compare(MustParse("1.0.0+a"), MustParse("1.0.0+b")) != 0 {
		t.Fatal("build metadata must not affect precedence")
	}
}
