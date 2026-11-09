// alexandriac: command-line compiler of eunomia.profiles/v1 YAML into the profile and space artefacts.
// "validate" reports N1–N5 findings; "compile" also writes the artefacts and space-lock.json (Parte II §12–§13).
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/ging/alexandria/internal/dsl"
)

// multi is a repeatable string flag.
type multi []string

// String renders the flag value.
func (m *multi) String() string { return strings.Join(*m, ",") }

// Set appends one value.
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "validate" && os.Args[1] != "compile") {
		fmt.Fprintln(os.Stderr, "usage: alexandriac validate|compile [-o dir] [-vendor dir] [-previous file]... file.yaml...")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fl := flag.NewFlagSet(cmd, flag.ExitOnError)
	out := fl.String("o", "out", "output directory (compile)")
	vendor := fl.String("vendor", "", "directory of vendored profiles: <slot>/<version>/vendor.json")
	var previous multi
	fl.Var(&previous, "previous", "YAML of the previous release, for the N4 version checks (repeatable)")
	_ = fl.Parse(os.Args[2:])
	srcs, err := read(fl.Args())
	if err != nil || len(srcs) == 0 {
		fmt.Fprintln(os.Stderr, "alexandriac: no input files", err)
		os.Exit(2)
	}
	prev, err := read(previous)
	if err != nil {
		fmt.Fprintln(os.Stderr, "alexandriac:", err)
		os.Exit(2)
	}
	opt := dsl.Options{Previous: prev}
	if *vendor != "" {
		opt.Vendor = dsl.FSVendor{FS: os.DirFS(*vendor)}
	}
	r := dsl.Compile(opt, srcs...)
	for _, f := range r.Findings.Sorted() {
		fmt.Fprintln(os.Stderr, f)
	}
	if !r.OK() {
		fmt.Fprintf(os.Stderr, "alexandriac: %d errors\n", len(r.Findings.Errors()))
		os.Exit(1)
	}
	if cmd == "validate" {
		fmt.Printf("ok: %d documents valid\n", len(r.Profiles)+map[bool]int{true: 1}[r.Space != nil])
		return
	}
	if err := write(*out, r); err != nil {
		fmt.Fprintln(os.Stderr, "alexandriac:", err)
		os.Exit(1)
	}
}

// read loads the named files.
func read(paths []string) ([]dsl.Source, error) {
	var out []dsl.Source
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		out = append(out, dsl.Source{Name: filepath.Base(p), Data: b})
	}
	return out, nil
}

// write stores every artefact under dir, plus space/space-lock.json when the bundle has a space.
func write(dir string, r *dsl.Result) error {
	files := map[string][]byte{}
	for _, a := range r.Artifacts {
		files[a.Path] = a.Bytes
	}
	if r.Lock != nil {
		files["space/space-lock.json"] = r.Lock.JSON()
	}
	for p, b := range files {
		full := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(full, b, 0o644); err != nil {
			return err
		}
		fmt.Println(full)
	}
	return nil
}
