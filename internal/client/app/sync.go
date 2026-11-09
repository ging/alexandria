// Package app implements Sync: obtains the listing, applies S1–S13, persists continuity and publishes a new snapshot atomically.
// Any failure keeps the previous snapshot and emits vhub.sync with its code; a bad package discards the whole listing (REQ-CLI-003..007).
package app

import (
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
)

// Cell holds the current snapshot; Store and Load are atomic.
type Cell struct {
	p atomic.Pointer[domain.Snapshot]
}

// Load returns the current snapshot, or nil before the first E1.
func (c *Cell) Load() *domain.Snapshot { return c.p.Load() }

// Store publishes a snapshot.
func (c *Cell) Store(s *domain.Snapshot) { c.p.Store(s) }

// Sync implements FL2.
type Sync struct {
	Hub        HubAPI
	Trust      *HubTrust
	Receipts   ReceiptVerifier
	Continuity ContinuityStore
	Packages   PackageStore
	Agreements *Agreements
	Audit      *Audit
	Status     *Status
	Cell       *Cell
	Clock      timex.Clock
	Events     EventSink
	Space      string

	mu          sync.Mutex
	accepted    *lifecycle.Accepted
	lastAttempt timex.Instant
	lastSuccess timex.Instant
	attempted   bool
	failed      bool
}

// Restore reloads persisted continuity and rebuilds the snapshot without re-verifying signatures.
func (s *Sync) Restore() error {
	c, err := s.Continuity.Load()
	if err != nil {
		return err
	}
	if c.DID != nil {
		s.Trust.Restore(c.DID, c.ResolvedAt)
	}
	if c.Accepted == nil {
		return nil
	}
	s.accepted = c.Accepted
	var l lifecycle.Listing
	raw, err := sign.ParseRaw(c.Accepted.Raw)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw.Signed, &l); err != nil {
		return err
	}
	pkgs := map[ni.Digest]*pkgv.Package{}
	for _, r := range lifecycle.KnownSlice(c.Accepted.Known) {
		f, ok, err := s.Packages.Get(r.Manifest)
		if err != nil || !ok {
			continue
		}
		if p, cerr := pkgv.Verify(f, r.Manifest, r.Version.String()); cerr == nil {
			pkgs[r.Manifest] = p
		}
	}
	s.Cell.Store(snapshot(l, c.Accepted.Known, pkgs))
	return nil
}

// Due reports whether a sync should run at now: sync_interval after a success, one minute after a failure.
func (s *Sync) Due(now timex.Instant) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.attempted {
		return true
	}
	if s.failed {
		retry := timex.Minute
		if iv := s.interval(); iv < retry {
			retry = iv
		}
		return now >= s.lastAttempt.Add(retry)
	}
	return now >= s.lastSuccess.Add(s.interval())
}

// interval is the sync_interval of the head package, or the default before any package.
func (s *Sync) interval() timex.Duration {
	if snap := s.Cell.Load(); snap != nil {
		return snap.Params.SyncInterval
	}
	return lifecycle.Defaults().SyncInterval
}

// RunIfDue runs a sync when it is due.
func (s *Sync) RunIfDue() {
	if s.Due(s.Clock.Now()) {
		_ = s.Run()
	}
}

// Run performs one synchronization and reports its failure code, if any.
func (s *Sync) Run() *codes.Error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.Clock.Now()
	s.attempted, s.lastAttempt = true, now
	err := s.run(now)
	s.failed = err != nil
	if err != nil {
		s.Events.Emit(notice(EvSync, s.Space, now, err.Code, err.Detail))
		return err
	}
	s.lastSuccess = now
	return nil
}

// run executes S1–S13 and the acceptance steps.
func (s *Sync) run(now timex.Instant) *codes.Error {
	resp, err := s.Hub.Listing()
	switch {
	case errors.Is(err, ErrUnauthorized):
		return codes.New(codes.SyncUnauthorized, "GET /v1/versions refused the credentials")
	case err != nil:
		return codes.New(codes.SyncUnreachable, "%v", err)
	}
	if resp.HasDate {
		s.Status.ObserveSkew(resp.Date, now)
	}
	raw, perr := sign.ParseRaw(resp.Body)
	if perr != nil {
		return codes.New(codes.SyncSignatureInvalid, "listing is not a signed envelope: %v", perr)
	}
	msg, perr := raw.Input()
	if perr != nil {
		return codes.New(codes.SyncSignatureInvalid, "listing signed part: %v", perr)
	}
	if cerr := s.Trust.Verify(msg, raw.Signature, codes.SyncSignatureInvalid); cerr != nil {
		return cerr
	}
	var l lifecycle.Listing
	if err := json.Unmarshal(raw.Signed, &l); err != nil || l.Type != lifecycle.ListingType {
		return codes.New(codes.SyncSignatureInvalid, "listing content: %v", err)
	}
	if now >= l.ExpiresAt {
		return codes.New(codes.SyncExpired, "expires_at %s is not after %s", l.ExpiresAt, now)
	}
	identical, cerr := lifecycle.CheckContinuity(s.accepted, l, resp.Body)
	if cerr != nil {
		return cerr
	}
	if identical {
		return nil
	}
	pkgs, fresh, cerr := s.packages(l)
	if cerr != nil {
		return cerr
	}
	if cerr := lifecycle.CheckRecords(l.Versions, func(v semver.Version) lifecycle.Params {
		r, _ := lifecycle.Find(l.Versions, v)
		return pkgs[r.Manifest].SpaceLock.Lifecycle
	}); cerr != nil {
		return cerr
	}
	return s.accept(l, resp.Body, pkgs, fresh)
}

// packages verifies every listed package not yet verified (S8–S13); one failure rejects the listing.
func (s *Sync) packages(l lifecycle.Listing) (map[ni.Digest]*pkgv.Package, []*pkgv.Package, *codes.Error) {
	have := map[ni.Digest]*pkgv.Package{}
	if snap := s.Cell.Load(); snap != nil {
		for d, p := range snap.Packages {
			have[d] = p
		}
	}
	out := map[ni.Digest]*pkgv.Package{}
	var fresh []*pkgv.Package
	for _, r := range l.Versions {
		if p, ok := have[r.Manifest]; ok && p.Manifest.Version == r.Version.String() {
			out[r.Manifest] = p
			continue
		}
		f, ok, err := s.Packages.Get(r.Manifest)
		if err == nil && ok {
			if p, cerr := pkgv.Verify(f, r.Manifest, r.Version.String()); cerr == nil {
				out[r.Manifest] = p
				continue
			}
		}
		f, err = s.Hub.Package(r.Manifest)
		if err != nil {
			return nil, nil, codes.New(codes.SyncUnreachable, "package %s: %v", r.Manifest, err)
		}
		p, cerr := s.verifyPackage(f, r)
		if cerr != nil {
			return nil, nil, cerr
		}
		out[r.Manifest] = p
		fresh = append(fresh, p)
	}
	return out, fresh, nil
}

// verifyPackage applies S8, S9, S10 and then S11–S13 to downloaded files.
func (s *Sync) verifyPackage(f pkgv.Files, r lifecycle.Record) (*pkgv.Package, *codes.Error) {
	if ni.Sum(f.Manifest) != r.Manifest {
		return nil, codes.New(codes.SyncManifestDigestMismatch, "manifest.json of %s does not match %s", r.Version, r.Manifest)
	}
	var sig sign.Signature
	if err := json.Unmarshal(f.Sig, &sig); err != nil {
		return nil, codes.New(codes.SyncManifestSignatureInvalid, "manifest.sig of %s: %v", r.Version, err)
	}
	if cerr := s.Trust.Verify(f.Manifest, sig, codes.SyncManifestSignatureInvalid); cerr != nil {
		return nil, cerr
	}
	if err := s.Receipts.Verify(r.Manifest, f.Receipt, s.Trust.Keys()); err != nil {
		return nil, codes.New(codes.SyncReceiptInvalid, "manifest.receipt of %s: %v", r.Version, err)
	}
	return pkgv.Verify(f, r.Manifest, r.Version.String())
}

// accept persists continuity, stores packages and publishes the snapshot (E1).
func (s *Sync) accept(l lifecycle.Listing, raw []byte, pkgs map[ni.Digest]*pkgv.Package, fresh []*pkgv.Package) *codes.Error {
	var prevKnown map[string]lifecycle.Record
	if s.accepted != nil {
		prevKnown = s.accepted.Known
	}
	for _, p := range fresh {
		if err := s.Packages.Put(p.Digest, p.Raw); err != nil {
			return codes.New(codes.SyncUnreachable, "storing package: %v", err)
		}
	}
	acc := &lifecycle.Accepted{Sequence: l.Sequence, Digest: ni.Sum(raw), Raw: raw, Known: lifecycle.Merge(prevKnown, l)}
	doc, at := s.Trust.Cache()
	if err := s.Continuity.Save(Continuity{Accepted: acc, DID: doc, ResolvedAt: at}); err != nil {
		return codes.New(codes.SyncUnreachable, "persisting continuity: %v", err)
	}
	// Packages kept for agreements stay usable even after their version leaves the listing.
	if prev := s.Cell.Load(); prev != nil {
		for d, p := range prev.Packages {
			if _, ok := pkgs[d]; !ok {
				pkgs[d] = p
			}
		}
	}
	s.accepted = acc
	snap := snapshot(l, acc.Known, pkgs)
	s.Cell.Store(snap)
	if revs := lifecycle.NewRevocations(prevKnown, l); len(revs) > 0 {
		s.Agreements.OnRevocations(revs, snap)
	}
	keep := s.Agreements.Retained()
	for _, r := range l.Versions {
		keep = append(keep, r.Manifest)
	}
	_ = s.Packages.Retain(keep)
	for _, p := range fresh {
		if r, ok := lifecycle.Find(l.Versions, semver.MustParse(p.Manifest.Version)); ok {
			s.Audit.OnNewVersion(r, snap)
		}
	}
	return nil
}

// snapshot builds the immutable snapshot of a listing.
func snapshot(l lifecycle.Listing, known map[string]lifecycle.Record, pkgs map[ni.Digest]*pkgv.Package) *domain.Snapshot {
	return &domain.Snapshot{
		Space: l.Space, Sequence: l.Sequence, IssuedAt: l.IssuedAt, ExpiresAt: l.ExpiresAt, Records: l.Versions,
		Known: known, Packages: pkgs, Params: domain.HeadParams(l.Versions, pkgs),
	}
}
