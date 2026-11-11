// Package app implements GC: every retention.gc_interval it deletes packages neither listed nor referenced by an agreement.
// It republishes the snapshot without them and emits vhub.gc; a deleted version that reappears is downloaded again (§4.10, REQ-CLI-017, REQ-CLI-033).
package app

import (
	"sort"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// GC collects packages.
type GC struct {
	Packages   PackageStore
	Agreements *Agreements
	Cell       *Cell
	Events     EventSink
	Clock      timex.Clock
	Namespace  string
	Interval   timex.Duration
	last       timex.Instant
	ran        bool
}

// RunIfDue runs a pass when gc_interval has elapsed since the last one.
func (g *GC) RunIfDue() {
	now := g.Clock.Now()
	if g.ran && now < g.last.Add(g.Interval) {
		return
	}
	g.Run()
}

// Run performs one collection pass and returns the deleted versions.
func (g *GC) Run() []string {
	now := g.Clock.Now()
	g.last, g.ran = now, true
	snap := g.Cell.Load()
	if snap == nil {
		return nil
	}
	keep := domain.Keep(snap.Records, g.Agreements.Retained())
	stored, _ := g.Packages.List()
	deleted := []string{}
	for _, d := range stored {
		if keep[d] {
			continue
		}
		if err := g.Packages.Delete(d); err == nil {
			deleted = append(deleted, version(snap, d))
		}
	}
	pkgs := map[ni.Digest]*pkgv.Package{}
	for d, p := range snap.Packages {
		if keep[d] {
			pkgs[d] = p
		}
	}
	next := *snap
	next.Packages = pkgs
	g.Cell.Store(&next)
	sort.Strings(deleted)
	n := notice(EvGC, g.Namespace, now, "", "package collection")
	n.Versions = deleted
	g.Events.Emit(n)
	return deleted
}

// version names the version of a stored manifest digest, or the digest itself.
func version(snap *domain.Snapshot, d ni.Digest) string {
	if p := snap.Packages[d]; p != nil {
		return p.Manifest.Version
	}
	for _, r := range snap.Known {
		if r.Manifest == d {
			return r.Version.String()
		}
	}
	return string(d)
}
