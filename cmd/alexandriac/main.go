// alexandriac: command-line compiler of eunomia.profiles/v1 YAML into the profile and space artefacts.
// "validate" reports N1–N5 findings; "compile" also writes the artefacts and space-lock.json (Parte II §12–§13).
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/dsl"
	"github.com/ging/alexandria/internal/telemetry"
)

// multi is a repeatable string flag.
type multi []string

// String renders the flag value.
func (m *multi) String() string { return strings.Join(*m, ",") }

// Set appends one value.
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

// main parses subcommand flags, compiles input YAML files, and reports findings or artefacts.
func main() {
	if len(os.Args) < 2 || (os.Args[1] != "validate" && os.Args[1] != "compile") {
		fmt.Fprintln(os.Stderr, "usage: alexandriac validate|compile [-o dir] [-vendor dir] [-previous file]... [-log-level lvl] [-log-format fmt] file.yaml...")
		os.Exit(2)
	}
	cmd := os.Args[1]
	fl := flag.NewFlagSet(cmd, flag.ExitOnError)
	out := fl.String("o", "out", "output directory (compile)")
	vendor := fl.String("vendor", "", "directory of vendored profiles: <slot>/<version>/vendor.json")
	logLevel := fl.String("log-level", "none", "structured log level on stderr (none, debug, info, warn, error)")
	logFormat := fl.String("log-format", "auto", "structured log format (auto, json, text)")
	var previous multi
	fl.Var(&previous, "previous", "YAML of the previous release, for the N4 version checks (repeatable)")
	_ = fl.Parse(os.Args[2:])

	ctx := context.Background()
	rt, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName: "alexandriac",
		Level:       *logLevel,
		Format:      *logFormat,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "alexandriac:", err)
		os.Exit(2)
	}
	defer func() { _ = rt.Shutdown(ctx) }()
	log := logx.Component(rt.Logger, "compiler")

	srcs, err := read(fl.Args())
	if err != nil || len(srcs) == 0 {
		log.Error("failed to load input files", logx.Err(err), slog.Int("files", len(fl.Args())))
		fmt.Fprintln(os.Stderr, "alexandriac: no input files", err)
		os.Exit(2)
	}
	prev, err := read(previous)
	if err != nil {
		log.Error("failed to load previous release files", logx.Err(err))
		fmt.Fprintln(os.Stderr, "alexandriac:", err)
		os.Exit(2)
	}
	opt := dsl.Options{Previous: prev}
	if *vendor != "" {
		opt.Vendor = dsl.FSVendor{FS: os.DirFS(*vendor)}
	}
	log.Debug("starting compilation", slog.String("subcommand", cmd), slog.Int("sources", len(srcs)), slog.Int("previous", len(prev)))
	r := dsl.Compile(opt, srcs...)
	for _, f := range r.Findings.Sorted() {
		fmt.Fprintln(os.Stderr, f)
	}
	if !r.OK() {
		log.Warn("compilation failed with findings", slog.Int("errors", len(r.Findings.Errors())), slog.Int("total_findings", len(r.Findings)))
		fmt.Fprintf(os.Stderr, "alexandriac: %d errors\n", len(r.Findings.Errors()))
		os.Exit(1)
	}
	if cmd == "validate" {
		docs := len(r.Profiles) + map[bool]int{true: 1}[r.Space != nil]
		log.Info("validation succeeded", slog.Int("documents", docs))
		fmt.Printf("ok: %d documents valid\n", docs)
		return
	}
	if err := write(*out, r); err != nil {
		log.Error("failed to write compiled artefacts", slog.String("output_dir", *out), logx.Err(err))
		fmt.Fprintln(os.Stderr, "alexandriac:", err)
		os.Exit(1)
	}
	log.Info("compilation succeeded", slog.String("output_dir", *out), slog.Int("artefacts", len(r.Artifacts)))
}

// read loads the named files.
func read(paths []string) ([]dsl.Source, error) {
	var out []dsl.Source
	for _, p := range paths {
		b, err := os.ReadFile(p) // #nosec G304 G703 -- input files named on the command line.
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
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil { // #nosec G301 G703 -- build output meant to be published.
			return err
		}
		if err := os.WriteFile(full, b, 0o644); err != nil { // #nosec G306 G703 -- build output meant to be published.
			return err
		}
		fmt.Println(full)
	}
	return nil
}
