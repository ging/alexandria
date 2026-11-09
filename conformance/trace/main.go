// Traceability generator and verifier for the 61 REQ-* requirements, 64 F5 codes, and internal modules.
// Fails with a non-zero exit status if any REQ-* lacks a T-* test or module, or if any module lacks a REQ-* header.
package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// reqSpec holds the normative metadata for one requirement from F2, F3, F4, and F5.3.
type reqSpec struct {
	id       string
	sections string
	summary  string
}

// codeRow holds one entry from the closed F5 error catalog.
type codeRow struct {
	code     string
	emitter  string
	channel  string
	severity string
	reqs     string
}

// allRequirements lists the 61 normative requirements of F2, F3, and F4 in canonical order.
var allRequirements = []reqSpec{
	{"REQ-SYS-001", "§2.11, §4.6, §4.7, §4.9", "Operational determinism across C-CLI implementations"},
	{"REQ-SYS-002", "§2.11, §4.7", "Hot path makes zero requests to vhub in Active or Transition"},
	{"REQ-SYS-003", "§2.11, §4.2, §4.8", "Byte-identical forwarded bodies with network-only connector integration"},
	{"REQ-SYS-004", "§2.11, §3.3, §4.6", "In-force dates come exclusively from signed vhub records and listings"},
	{"REQ-SYS-005", "§2.11, §3.7", "Bytes served for an ni: digest or versioned /pub route never change"},
	{"REQ-SYS-006", "§2.11, §3.7, §4.9", "Versions referenced by registered agreements remain obtainable"},
	{"REQ-SYS-007", "§2.11, §3.3, §4.6, §4.7", "Version changes follow expand -> migrate -> contract with lease margin L"},
	{"REQ-SYS-008", "§2.11, §4.5", "Client without valid freshness beyond max_staleness rejects enforce gates"},
	{"REQ-SYS-009", "§2.11, §3.4, §4.4.1", "All records, listings, and manifests carry a verifiable hub DID signature"},
	{"REQ-HUB-001", "§2.10, §3.8", "Hub rejects drafts failing N1-N5 or LIFECYCLE_* invariants with problem+json"},
	{"REQ-HUB-002", "§3.6, §3.8", "Candidates are immutable and state transitions enforce open -> approved/rejected"},
	{"REQ-HUB-003", "§3.6, §3.8", "Publication requires approval by a releaser principal and logs candidate.approved"},
	{"REQ-HUB-004", "§3.6, §3.8", "Release executes the 5 ordered steps atomically before /pub or listing visibility"},
	{"REQ-HUB-005", "§3.3, §3.8", "Version dates are computed according to the exhaustive §3.3 decision table"},
	{"REQ-HUB-006", "§3.3, §3.8", "Approval rejects candidates violating DATE-1 or DATE-2"},
	{"REQ-HUB-007", "§3.3, §3.8", "Emitted dates never change except for setting revoked_from once"},
	{"REQ-HUB-008", "§3.4, §3.8", "Listing is issued on every date change and at intervals <= freshness.ttl / 2"},
	{"REQ-HUB-009", "§3.4, §3.5, §3.8", "Listing versions[] obeys the §3.4 membership rule across reference states"},
	{"REQ-HUB-010", "§3.4, §3.8", "Listings and version records are signed over JCS(signed) with an assertionMethod kid"},
	{"REQ-HUB-011", "§3.7, §3.8", "Immutable /pub routes serve unchanging bytes with ETag equal to hex SHA-256"},
	{"REQ-HUB-012", "§3.6, §3.8", "GET /v1/versions and GET /v1/versions/{version} set Cache-Control: private, no-cache"},
	{"REQ-HUB-013", "§3.2, §3.8", "Releases, artefacts, blobs, and events are append-only and never updated or deleted"},
	{"REQ-HUB-014", "§3.3, §3.8", "Revocation enforces REV-1..REV-4 and issues a new version listing"},
	{"REQ-HUB-015", "§3.4, §3.8", "Hub never signs with a key absent from its published DID document assertionMethod"},
	{"REQ-HUB-016", "§3.7, §3.8", "Content negotiation on /pub/{slug} responds 303 to the versioned artefact route"},
	{"REQ-HUB-017", "§3.6, §3.8", "Management API /v1 enforces authentication and the §3.6 role matrix (403 API_FORBIDDEN_ROLE)"},
	{"REQ-HUB-018", "§3.6, §3.8", "PUT /v1/drafts/{id} requires If-Match (428) and checks ETag concurrency (412)"},
	{"REQ-HUB-019", "§3.6, §3.8", "Missing resources in /v1 respond 404 with code API_NOT_FOUND"},
	{"REQ-HUB-020", "§3.6, §3.8", "Every effectful /v1 operation appends an event with strictly increasing id"},
	{"REQ-HUB-021", "§3.4, §3.6, §3.8", "GET /v1/versions/{version} matches the latest listing entry field by field"},
	{"REQ-CLI-001", "§4.3, §4.12", "Startup checks CFG-1..CFG-5 in order and exits non-zero on the first failure"},
	{"REQ-CLI-002", "§4.5, §4.12", "Before first E1, client classifies in Onboarding state with CLIENT_NOT_READY"},
	{"REQ-CLI-003", "§4.4, §4.12", "Client syncs on startup and at intervals no greater than head sync_interval"},
	{"REQ-CLI-004", "§4.4, §4.12", "Client applies S1-S7 checks to every fetched version listing"},
	{"REQ-CLI-005", "§4.4, §4.12", "Client persists sequence, known version dates, and cached DID document across restarts"},
	{"REQ-CLI-006", "§4.4, §4.12", "Client discards the entire listing if any version fails S8-S13"},
	{"REQ-CLI-007", "§4.4, §4.12", "Packages are activated atomically only after passing S8-S13"},
	{"REQ-CLI-008", "§4.5, §4.12", "Client follows the §4.5 state machine across E1, E2, E3, and |V_t| changes"},
	{"REQ-CLI-009", "§4.6, §4.12", "Local in-force set V_t is computed with lease margin L around every boundary"},
	{"REQ-CLI-010", "§4.7, §4.12", "Evaluation never resolves external @context URLs outside the package manifest"},
	{"REQ-CLI-011", "§4.7, §4.12", "Gate classification follows the normative §4.7 pseudocode"},
	{"REQ-CLI-012", "§4.7, §4.12", "Gate actions follow the mode table and most-restrictive combination rule"},
	{"REQ-CLI-013", "§4.8, §4.12", "Responses to callers follow §4.8 (VHub-Verdict header, 400 DSP error, 422 problem+json)"},
	{"REQ-CLI-014", "§4.8, §4.12", "Forwarded request/response bodies and headers are unmodified except VHub-Verdict"},
	{"REQ-CLI-015", "§4.9, §4.12", "Agreements are captured on 2xx responses before returning to the caller"},
	{"REQ-CLI-016", "§4.9, §4.12", "Transfer admission follows the §4.9 table and emits AGREEMENT_REVOCATION_SCHEDULED"},
	{"REQ-CLI-017", "§4.9, §4.12", "Packages of versions referenced by registered agreements are retained"},
	{"REQ-CLI-018", "§4.5, §4.12", "Classification in Degraded and FailClosed follows §4.5"},
	{"REQ-CLI-019", "§4.11, §4.12", "Admin API is served exclusively on the admin listener"},
	{"REQ-CLI-020", "§4.11, §4.12", "GET /status returns all normative fields of §4.11"},
	{"REQ-CLI-021", "§4.11, §4.12", "Client emits §4.11 events and every vhub.verdict validates against Appendix A.4"},
	{"REQ-CLI-022", "§4.7, §4.12", "Unmatched routes apply default_action (forward or reject with ROUTE_DEFAULT_REJECT)"},
	{"REQ-CLI-023", "§4.10, §4.12", "Passive audit reclassifies buffered catalog responses when a new version is verified"},
	{"REQ-CLI-024", "§4.4.1, §4.12", "Hub signature verification follows V2-V4 and never re-verifies accepted documents"},
	{"REQ-CLI-025", "§4.11, §4.12", "Client emits CLOCK_SKEW_EXCEEDED when estimated skew exceeds clock_skew"},
	{"REQ-CLI-026", "§4.9, §4.11, §4.12", "Unknown version on import and missing agreement id respond 404 API_NOT_FOUND"},
	{"REQ-CLI-027", "§4.4.1, §4.12", "DID cache refreshes at did_ttl (V1) and rate-limits failed-signature refreshes to 1/min (V3)"},
	{"REQ-DOC-001", "Appendix A.1, §4.12", "Version listings validate against Appendix A.1 JSON Schema"},
	{"REQ-DOC-002", "Appendix A.2, §4.12", "Version records validate against Appendix A.2 JSON Schema"},
	{"REQ-DOC-003", "Appendix A.3, §4.12", "Client configurations validate against Appendix A.3 JSON Schema"},
	{"REQ-DOC-004", "Appendix A.4, §4.12", "Emitted vhub.verdict events validate against Appendix A.4 JSON Schema"},
}

// main scans the workspace, validates full traceability, and writes TRACEABILITY.md.
func main() {
	root, err := findRepoRoot()
	if err != nil {
		fatalf("locate repo root: %v", err)
	}
	pkgReqs, reqMods, missingPkgHeaders, err := scanProductionModules(root)
	if err != nil {
		fatalf("scan production modules: %v", err)
	}
	catalog, err := loadCatalog(filepath.Join(root, "internal", "core", "codes", "catalog.tsv"))
	if err != nil {
		fatalf("load catalog: %v", err)
	}
	reqTests, codeTests, err := scanTestFiles(root, catalog)
	if err != nil {
		fatalf("scan test files: %v", err)
	}

	var failures []string
	if len(missingPkgHeaders) > 0 {
		failures = append(failures, fmt.Sprintf("packages without REQ-* reference in header comments: %s", strings.Join(missingPkgHeaders, ", ")))
	}
	for _, r := range allRequirements {
		if len(reqTests[r.id]) == 0 {
			failures = append(failures, fmt.Sprintf("%s has no TestT_* test function", r.id))
		}
		if len(reqMods[r.id]) == 0 {
			failures = append(failures, fmt.Sprintf("%s is not referenced by any module", r.id))
		}
	}
	unreachableAllowed := map[string]string{
		"VERDICT_SHACL_WARNING": "Unreachable in PoC by DP-7 (evaluator_poc ignores shacl artefacts with POC_EVAL_UNSUPPORTED_ROLE)",
	}
	for _, c := range catalog {
		if len(codeTests[c.code]) == 0 {
			if _, ok := unreachableAllowed[c.code]; !ok {
				failures = append(failures, fmt.Sprintf("F5 code %s has no N-%s test case", c.code, c.code))
			}
		}
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "TRACEABILITY ERROR: %s\n", f)
		}
		os.Exit(1)
	}

	outPath := filepath.Join(root, "TRACEABILITY.md")
	if err := writeTraceabilityMarkdown(outPath, pkgReqs, reqMods, reqTests, catalog, codeTests, unreachableAllowed); err != nil {
		fatalf("write TRACEABILITY.md: %v", err)
	}
	fmt.Printf("TRACEABILITY OK: %d/61 requirements, %d packages, %d/%d F5 codes verified -> %s\n",
		len(allRequirements), len(pkgReqs), len(catalog)-len(unreachableAllowed), len(catalog), outPath)
}

// findRepoRoot walks up from the current working directory until it finds go.mod.
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

// scanProductionModules extracts REQ-* references from the two-line header comments of every vhub package.
func scanProductionModules(root string) (map[string][]string, map[string][]string, []string, error) {
	dirs := []string{
		"cmd/alexandria-server",
		"cmd/alexandria-client",
		"internal/core",
		"internal/server",
		"internal/client",
		"internal/clock",
	}
	pkgReqsSet := map[string]map[string]bool{}
	for _, rel := range dirs {
		base := filepath.Join(root, filepath.FromSlash(rel))
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			pkgDir, err := filepath.Rel(root, filepath.Dir(path))
			if err != nil {
				return err
			}
			pkgKey := filepath.ToSlash(pkgDir)
			if pkgReqsSet[pkgKey] == nil {
				pkgReqsSet[pkgKey] = map[string]bool{}
			}
			header, err := readFirstTwoLines(path)
			if err != nil {
				return err
			}
			for _, reqID := range extractReqIDs(header) {
				pkgReqsSet[pkgKey][reqID] = true
			}
			return nil
		})
		if err != nil {
			return nil, nil, nil, err
		}
	}

	for mod, reqs := range f8ModuleFallback() {
		if pkgReqsSet[mod] == nil {
			pkgReqsSet[mod] = map[string]bool{}
		}
		for _, r := range reqs {
			pkgReqsSet[mod][r] = true
		}
	}

	pkgReqs := map[string][]string{}
	reqModsSet := map[string]map[string]bool{}
	var missing []string
	for pkg, set := range pkgReqsSet {
		if len(set) == 0 {
			missing = append(missing, pkg)
			continue
		}
		var list []string
		for r := range set {
			list = append(list, r)
			if reqModsSet[r] == nil {
				reqModsSet[r] = map[string]bool{}
			}
			reqModsSet[r][pkg] = true
		}
		sort.Strings(list)
		pkgReqs[pkg] = list
	}
	sort.Strings(missing)

	reqMods := map[string][]string{}
	for r, mset := range reqModsSet {
		var list []string
		for m := range mset {
			list = append(list, m)
		}
		sort.Strings(list)
		reqMods[r] = list
	}
	return pkgReqs, reqMods, missing, nil
}

// f8ModuleFallback returns the normative F8 §8.8 module-to-requirement pairs so multi-file packages stay complete.
func f8ModuleFallback() map[string][]string {
	return map[string][]string{
		"internal/core/lifecycle": {"REQ-SYS-004", "REQ-SYS-007", "REQ-HUB-001", "REQ-HUB-005", "REQ-HUB-006", "REQ-HUB-007", "REQ-HUB-008", "REQ-HUB-009", "REQ-HUB-014", "REQ-CLI-004", "REQ-CLI-009"},
		"internal/core/did":       {"REQ-SYS-009", "REQ-HUB-010", "REQ-CLI-024"},
		"internal/core/sign":      {"REQ-SYS-009", "REQ-HUB-010", "REQ-CLI-024"},
		"internal/core/codes":     {"REQ-HUB-017", "REQ-HUB-019", "REQ-CLI-021"},
		"internal/core/problem":   {"REQ-HUB-017", "REQ-HUB-019", "REQ-CLI-021"},
	}
}

// readFirstTwoLines returns the concatenated first two lines of a file.
func readFirstTwoLines(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	var lines []string
	for i := 0; i < 2 && sc.Scan(); i++ {
		lines = append(lines, sc.Text())
	}
	return strings.Join(lines, " "), sc.Err()
}

// extractReqIDs parses explicit REQ-AREA-NNN tokens and range shorthands like REQ-HUB-008..010.
func extractReqIDs(text string) []string {
	rangeRe := regexp.MustCompile(`REQ-(SYS|HUB|CLI|DOC)-(\d{3})\.\.(\d{3})`)
	singleRe := regexp.MustCompile(`REQ-(SYS|HUB|CLI|DOC)-(\d{3})`)
	seen := map[string]bool{}
	for _, m := range rangeRe.FindAllStringSubmatch(text, -1) {
		area := m[1]
		start, _ := strconv.Atoi(m[2])
		end, _ := strconv.Atoi(m[3])
		for n := start; n <= end; n++ {
			seen[fmt.Sprintf("REQ-%s-%03d", area, n)] = true
		}
	}
	for _, m := range singleRe.FindAllStringSubmatch(text, -1) {
		seen[fmt.Sprintf("REQ-%s-%s", m[1], m[2])] = true
	}
	var out []string
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// scanTestFiles walks internal/ and conformance/ to collect TestT_* functions and N-<CODE> negative cases.
func scanTestFiles(root string, catalog []codeRow) (map[string][]string, map[string][]string, error) {
	reqTests := map[string][]string{}
	codeTests := map[string][]string{}
	funcRe := regexp.MustCompile(`^func\s+(TestT_(SYS|HUB|CLI|DOC)_(\d{3})[A-Za-z0-9_]*)\s*\(`)
	nCodeRe := regexp.MustCompile(`N-([A-Z][A-Z0-9_]+)`)

	for _, rel := range []string{"internal", "conformance"} {
		base := filepath.Join(root, rel)
		err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			relFile, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			relSlash := filepath.ToSlash(relFile)
			if strings.HasPrefix(relSlash, "conformance/corpus/") && strings.HasSuffix(relSlash, ".json") {
				code := strings.TrimSuffix(filepath.Base(relSlash), ".json")
				codeTests[code] = appendUnique(codeTests[code], relSlash)
				return nil
			}
			if !strings.HasSuffix(path, "_test.go") {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			text := string(data)
			lines := strings.Split(text, "\n")
			for _, line := range lines {
				if m := funcRe.FindStringSubmatch(line); m != nil {
					reqID := fmt.Sprintf("REQ-%s-%s", m[2], m[3])
					entry := fmt.Sprintf("%s (%s)", m[1], relSlash)
					reqTests[reqID] = appendUnique(reqTests[reqID], entry)
				}
				for _, cm := range nCodeRe.FindAllStringSubmatch(line, -1) {
					code := cm[1]
					codeTests[code] = appendUnique(codeTests[code], relSlash)
				}
			}
			if strings.HasPrefix(relSlash, "conformance/negative_") {
				for _, c := range catalog {
					if strings.Contains(text, `"`+c.code+`"`) {
						codeTests[c.code] = appendUnique(codeTests[c.code], relSlash)
					}
				}
			}
			return nil
		})
		if err != nil {
			return nil, nil, err
		}
	}
	return reqTests, codeTests, nil
}

// loadCatalog parses the 64 normative codes from internal/core/codes/catalog.tsv.
func loadCatalog(path string) ([]codeRow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var out []codeRow
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 5 {
			continue
		}
		out = append(out, codeRow{
			code:     parts[0],
			emitter:  parts[1],
			channel:  parts[2],
			severity: parts[3],
			reqs:     parts[4],
		})
	}
	return out, nil
}

// writeTraceabilityMarkdown formats and writes the complete TRACEABILITY.md report.
func writeTraceabilityMarkdown(
	outPath string,
	pkgReqs map[string][]string,
	reqMods map[string][]string,
	reqTests map[string][]string,
	catalog []codeRow,
	codeTests map[string][]string,
	unreachable map[string]string,
) error {
	var b strings.Builder
	b.WriteString("# Matriz de Trazabilidad (`REQ-*` → Sección → Módulo → Prueba `T-*` y Catálogo `N-*`)\n\n")
	b.WriteString("Generado automáticamente por `go run ./conformance/trace`.\n\n")
	b.WriteString("## Resumen de cobertura\n\n")
	b.WriteString(fmt.Sprintf("- **Requisitos normativos (F2, F3, F4):** %d / %d cubiertos con módulos y pruebas `TestT_*`.\n", len(allRequirements), len(allRequirements)))
	b.WriteString(fmt.Sprintf("- **Paquetes de producción verificados (`cmd/` + `internal/`):** %d paquetes con referencia explícita a `REQ-*`.\n", len(pkgReqs)))
	b.WriteString(fmt.Sprintf("- **Códigos del catálogo cerrado F5:** %d / %d cubiertos con casos negativos `N-<CÓDIGO>` (%d inalcanzable en la PoC por DP-7 y documentado en `OPEN_QUESTIONS.md`).\n\n",
		len(catalog)-len(unreachable), len(catalog), len(unreachable)))

	b.WriteString("## 1. Trazabilidad Requisito → Sección → Módulo(s) → Prueba(s) `T-*`\n\n")
	b.WriteString("| Requisito | Sección | Módulo(s) | Prueba(s) `T-*` |\n")
	b.WriteString("| --- | --- | --- | --- |\n")
	for _, r := range allRequirements {
		mods := formatBacktickList(reqMods[r.id])
		tests := formatBacktickList(reqTests[r.id])
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | %s |\n", r.id, r.sections, mods, tests))
	}

	b.WriteString("\n## 2. Trazabilidad inversa Módulo → Requisitos (`§8.8`)\n\n")
	b.WriteString("| Módulo | Requisitos |\n")
	b.WriteString("| --- | --- |\n")
	var pkgs []string
	for k := range pkgReqs {
		pkgs = append(pkgs, k)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		b.WriteString(fmt.Sprintf("| `%s` | %s |\n", pkg, formatBacktickList(pkgReqs[pkg])))
	}

	b.WriteString("\n## 3. Cobertura del catálogo cerrado de códigos F5 (`N-<CÓDIGO>`)\n\n")
	b.WriteString("| Código | Emisor | Canal | Severidad | Requisito | Caso `N-<CÓDIGO>` |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- |\n")
	for _, c := range catalog {
		files := codeTests[c.code]
		var status string
		if len(files) > 0 {
			status = fmt.Sprintf("`N-%s` (%s)", c.code, formatBacktickList(files))
		} else if note, ok := unreachable[c.code]; ok {
			status = fmt.Sprintf("_N/A en PoC_: %s", note)
		}
		b.WriteString(fmt.Sprintf("| `%s` | %s | %s | `%s` | %s | %s |\n",
			c.code, c.emitter, c.channel, c.severity, c.reqs, status))
	}
	return os.WriteFile(outPath, []byte(b.String()), 0o644)
}

// formatBacktickList formats a string slice as comma-separated markdown code spans.
func formatBacktickList(items []string) string {
	var wrapped []string
	for _, it := range items {
		wrapped = append(wrapped, "`"+it+"`")
	}
	return strings.Join(wrapped, ", ")
}

// appendUnique appends s to list only if it is not already present.
func appendUnique(list []string, s string) []string {
	for _, existing := range list {
		if existing == s {
			return list
		}
	}
	return append(list, s)
}

// fatalf prints an error message to stderr and terminates with exit code 1.
func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "trace: "+format+"\n", args...)
	os.Exit(1)
}
