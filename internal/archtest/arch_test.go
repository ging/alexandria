// Architecture test: dependencies point inward (adapters → app → domain → core) and server/client stay apart.
// Also enforces the DP-1 dependency allow-list and that only internal/clock reads the wall clock (DP-1, DP-9).
package archtest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// module is the import path prefix of this repository.
const module = "github.com/ging/alexandria/"

// root is the repository root relative to this test.
var root = filepath.Join("..", "..")

// allowedExternal lists the only non-standard modules production code may import (DP-1).
var allowedExternal = []string{"modernc.org/sqlite", "github.com/santhosh-tekuri/jsonschema/v6", "gopkg.in/yaml.v3", "github.com/piprate/json-gold"}

// pkgFile is one production Go file with its package directory and imports.
type pkgFile struct {
	dir, path string
	imports   []string
	src       string
}

// productionFiles parses every non-test Go file under the vhub packages in internal/ and cmd/.
func productionFiles(t *testing.T) []pkgFile {
	t.Helper()
	var out []pkgFile
	for _, top := range []string{"internal/core", "internal/dsl", "internal/server", "internal/client", "internal/clock", "cmd/alexandria-server", "cmd/alexandria-client", "cmd/alexandriac"} {
		_ = filepath.Walk(filepath.Join(root, top), func(p string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			f, perr := parser.ParseFile(token.NewFileSet(), p, nil, parser.ImportsOnly)
			if perr != nil {
				t.Fatal(perr)
			}
			src, _ := os.ReadFile(p)
			rel, _ := filepath.Rel(root, filepath.Dir(p))
			pf := pkgFile{dir: filepath.ToSlash(rel), path: p, src: string(src)}
			for _, imp := range f.Imports {
				s, _ := strconv.Unquote(imp.Path.Value)
				pf.imports = append(pf.imports, s)
			}
			out = append(out, pf)
			return nil
		})
	}
	return out
}

// layerOf classifies a package directory into core, domain, app, adapters, cmd or other.
func layerOf(dir string) string {
	switch {
	case strings.HasPrefix(dir, "internal/core"):
		return "core"
	case strings.HasPrefix(dir, "internal/dsl"):
		return "lang"
	case strings.Contains(dir, "/domain"):
		return "domain"
	case strings.Contains(dir, "/app"):
		return "app"
	case strings.Contains(dir, "/adapters"):
		return "adapters"
	case strings.HasPrefix(dir, "cmd"):
		return "cmd"
	}
	return "other"
}

// sideOf returns server, client or empty for shared code.
func sideOf(dir string) string {
	switch {
	case strings.HasPrefix(dir, "internal/server"):
		return "server"
	case strings.HasPrefix(dir, "internal/client"):
		return "client"
	}
	return ""
}

func TestLayersPointInward(t *testing.T) {
	forbidden := map[string][]string{
		"core":   {"/domain", "/app", "/adapters", "internal/clock", "internal/testkit", "internal/server", "internal/client"},
		"domain": {"/app", "/adapters", "internal/clock", "internal/testkit"},
		"app":    {"/adapters", "internal/clock", "internal/testkit"},
		"lang":   {"internal/server", "internal/client", "internal/clock", "internal/testkit"},
	}
	for _, f := range productionFiles(t) {
		for _, imp := range f.imports {
			local, ok := strings.CutPrefix(imp, module)
			if !ok {
				continue
			}
			for _, bad := range forbidden[layerOf(f.dir)] {
				if strings.Contains(local, bad) {
					t.Errorf("%s (%s) imports %s", f.path, layerOf(f.dir), imp)
				}
			}
		}
	}
}

func TestServerAndClientDoNotImportEachOther(t *testing.T) {
	for _, f := range productionFiles(t) {
		side := sideOf(f.dir)
		for _, imp := range f.imports {
			local := strings.TrimPrefix(imp, module)
			if (side == "server" && strings.HasPrefix(local, "internal/client")) || (side == "client" && strings.HasPrefix(local, "internal/server")) {
				t.Errorf("%s imports %s", f.path, imp)
			}
		}
	}
}

func TestNoProductionImportOfTestkit(t *testing.T) {
	for _, f := range productionFiles(t) {
		for _, imp := range f.imports {
			if strings.HasSuffix(imp, "internal/testkit") {
				t.Errorf("%s imports testkit", f.path)
			}
		}
	}
}

func TestOnlyAllowedExternalModules(t *testing.T) {
	for _, f := range productionFiles(t) {
		for _, imp := range f.imports {
			first := strings.Split(imp, "/")[0]
			if !strings.Contains(first, ".") || strings.HasPrefix(imp, module) {
				continue
			}
			ok := false
			for _, a := range allowedExternal {
				if strings.HasPrefix(imp, a) {
					ok = true
				}
			}
			if !ok {
				t.Errorf("%s imports %s, outside DP-1", f.path, imp)
			}
		}
	}
}

func TestOnlyClockReadsWallTime(t *testing.T) {
	for _, f := range productionFiles(t) {
		if f.dir == "internal/clock" {
			continue
		}
		if strings.Contains(f.src, "time.Now(") || strings.Contains(f.src, "time.Since(") {
			t.Errorf("%s reads the wall clock; use timex.Clock (DP-9)", f.path)
		}
	}
}
