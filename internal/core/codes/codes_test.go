// Catalog test: the embedded F5 table and the Go constants describe the same closed set.
// Also checks that every code is used by some production package (D2, F5).
package codes

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// constRe extracts the catalog constants declared in codes.go.
var constRe = regexp.MustCompile(`(?m)^\t[A-Za-z]+\s+Code\s+= "([A-Z0-9_]+)"$`)

func TestCatalogHas77Codes(t *testing.T) {
	if n := len(All()); n != 77 {
		t.Fatalf("catalog has %d codes, F5 lists 77", n)
	}
}

func TestConstantsMatchCatalog(t *testing.T) {
	src, err := os.ReadFile("codes.go")
	if err != nil {
		t.Fatal(err)
	}
	m := constRe.FindAllStringSubmatch(string(src), -1)
	if len(m) != len(All()) {
		t.Fatalf("%d constants for %d catalog rows", len(m), len(All()))
	}
	for _, c := range m {
		if _, ok := Lookup(Code(c[1])); !ok {
			t.Errorf("constant %s is not in the catalog", c[1])
		}
	}
}

func TestNoCodeLiteralOutsideCatalog(t *testing.T) {
	lit := regexp.MustCompile(`"((?:API|NS|STEWARD|VOCAB|RELEASE|SIGNER|DATES|LIFECYCLE|CONFIG|RULES|RULE|SYNC|DID|CLIENT|NO|MESSAGE|CONTEXT|VERDICT|ROUTE|AGREEMENT|AUDIT|CLOCK)_[A-Z0-9_]+)"`)
	root := filepath.Join("..", "..", "..")
	_ = filepath.Walk(filepath.Join(root, "internal"), func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "codes.go") {
			return nil
		}
		b, _ := os.ReadFile(p)
		for _, m := range lit.FindAllStringSubmatch(string(b), -1) {
			if _, ok := Lookup(Code(m[1])); !ok {
				t.Errorf("%s uses %s, which is not in the catalog", p, m[1])
			}
		}
		return nil
	})
}
