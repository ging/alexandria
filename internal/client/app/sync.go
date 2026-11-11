// Package app implements Sync: obtains the namespace listing, applies S1–S13, persists continuity and publishes a new snapshot atomically.
// Any failure keeps the previous snapshot and emits vhub.sync with its code; a bad package discards the whole listing (REQ-CLI-003..007, REQ-CLI-034).
package app

import (
	"encoding/json"
	"errors"
	"log/slog"
	"slices"
	"sync"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/schemas"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
)

// Sync implements FL2.
type Sync struct {
	Hub        HubAPI
	Trust      *HubTrust
	Continuity ContinuityStore
	Packages   PackageStore
	Agreements *Agreements
	Audit      *Audit
	Status     *Status
	Cell       *Cell
	Clock      timex.Clock
	Events     EventSink
	Logger     *slog.Logger
	Namespace  string
	HubDID     did.DID
	Context    string

	mu          sync.Mutex
	accepted    *lifecycle.Accepted
	lastAttempt timex.Instant
	lastSuccess timex.Instant
	attempted   bool
	failed      bool
}

// listingCredential is the decoded listing.
type listingCredential = vc.Credential[lifecycle.ListingSubject]

// Restore reloads persisted continuity and rebuilds the snapshot without re-verifying proofs (§4.4.1).
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
	var l listingCredential
	if err := json.Unmarshal(c.Accepted.Raw, &l); err != nil {
		return err
	}
	pkgs := map[ni.Digest]*pkgv.Package{}
	for _, r := range lifecycle.KnownSlice(c.Accepted.Known) {
		f, ok, err := s.Packages.Get(r.Manifest)
		if err != nil || !ok {
			continue
		}
		if p, cerr := pkgv.Verify(f, r.Manifest, s.Namespace, r.Version.String()); cerr == nil {
			pkgs[r.Manifest] = p
		}
	}
	s.Cell.Store(snapshot(l, c.Accepted.Known, pkgs))
	logx.Default(s.Logger).Info("restored snapshot from continuity", slog.String("namespace", s.Namespace), slog.Uint64("sequence", c.Accepted.Sequence), slog.Int("packages", len(pkgs)))
	return nil
}

// Due reports whether a sync should run: sync_interval after a success, one minute after a failure (REQ-CLI-003).
func (s *Sync) Due(now timex.Instant) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.attempted {
		return true
	}
	if s.failed {
		retry := min(timex.Minute, s.interval())
		return now >= s.lastAttempt.Add(retry)
	}
	return now >= s.lastSuccess.Add(s.interval()) || now < s.lastSuccess
}

// interval is the sync_interval of the head manifest, or the default before any package.
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
		logx.Default(s.Logger).Warn("hub synchronization failed", slog.String("namespace", s.Namespace), logx.Err(err))
		s.Events.Emit(notice(EvSync, s.Namespace, now, err.Code, err.Detail))
		return err
	}
	s.lastSuccess = now
	return nil
}

// run executes S1–S13 and the acceptance steps.
func (s *Sync) run(now timex.Instant) *codes.Error {
	resp, err := s.Hub.Listing()
	switch {
	case errors.Is(err, ErrNamespaceNotFound):
		return codes.New(codes.SyncNamespaceNotFound, "the hub has no namespace %s", s.Namespace)
	case err != nil:
		return codes.New(codes.SyncUnreachable, "%v", err)
	}
	if resp.HasDate {
		s.Status.ObserveSkew(resp.Date, now)
	}
	l, cerr := s.checkListing(resp.Body)
	if cerr != nil {
		return cerr
	}
	if now >= *l.ValidUntil {
		return codes.New(codes.SyncExpired, "validUntil %s is not after %s", l.ValidUntil, now)
	}
	identical, cerr := lifecycle.CheckContinuity(s.accepted, l.Subject, resp.Body)
	if cerr != nil || identical {
		return cerr
	}
	pkgs, fresh, cerr := s.packages(l.Subject)
	if cerr != nil {
		return cerr
	}
	if cerr := lifecycle.CheckRecords(l.Subject.Versions, func(v semver.Version) lifecycle.Params {
		r, _ := lifecycle.Find(l.Subject.Versions, v)
		return pkgs[r.Manifest].Manifest.Lifecycle
	}); cerr != nil {
		return cerr
	}
	return s.accept(l, resp.Body, pkgs, fresh)
}

// checkListing applies S2: schema A.1, @context, issuer, namespace and the hub proof.
func (s *Sync) checkListing(raw []byte) (listingCredential, *codes.Error) {
	var l listingCredential
	if err := schemas.Validate(schemas.Versions, raw); err != nil {
		return l, codes.New(codes.SyncMalformed, "listing: %v", err)
	}
	if err := json.Unmarshal(raw, &l); err != nil {
		return l, codes.New(codes.SyncMalformed, "listing: %v", err)
	}
	if cerr := s.checkShape(l.Context, l.Issuer, l.Subject.Namespace); cerr != nil {
		return l, cerr
	}
	return l, s.Trust.Verify(raw, codes.SyncSignatureInvalid)
}

// checkShape compares @context, issuer and namespace with the trust configuration (REQ-CLI-034).
func (s *Sync) checkShape(ctx []string, issuer, ns string) *codes.Error {
	switch {
	case !slices.Equal(ctx, []string{vc.CredentialsV2, s.Context}):
		return codes.New(codes.SyncMalformed, "@context %v is not [credentials/v2, %s]", ctx, s.Context)
	case issuer != string(s.HubDID):
		return codes.New(codes.SyncMalformed, "issuer %s is not %s", issuer, s.HubDID)
	case ns != s.Namespace:
		return codes.New(codes.SyncMalformed, "namespace %s is not %s", ns, s.Namespace)
	}
	return nil
}

// packages verifies every listed package not yet verified (S8–S13); one failure rejects the listing.
func (s *Sync) packages(l lifecycle.ListingSubject) (map[ni.Digest]*pkgv.Package, []*pkgv.Package, *codes.Error) {
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
		if f, ok, err := s.Packages.Get(r.Manifest); err == nil && ok {
			if p, cerr := pkgv.Verify(f, r.Manifest, s.Namespace, r.Version.String()); cerr == nil {
				out[r.Manifest] = p
				continue
			}
		}
		f, err := s.Hub.Package(r.Manifest)
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

// verifyPackage applies S8, S9 and S11–S13 to downloaded files.
func (s *Sync) verifyPackage(f pkgv.Files, r lifecycle.Record) (*pkgv.Package, *codes.Error) {
	if ni.Sum(f.Manifest) != r.Manifest {
		return nil, codes.New(codes.SyncManifestDigestMismatch, "manifest.json of %s does not match %s", r.Version, r.Manifest)
	}
	if err := schemas.Validate(schemas.Manifest, f.Manifest); err != nil {
		return nil, codes.New(codes.SyncMalformed, "manifest of %s: %v", r.Version, err)
	}
	c, err := pkgv.Parse(f.Manifest)
	if err != nil {
		return nil, codes.New(codes.SyncMalformed, "manifest of %s: %v", r.Version, err)
	}
	// The manifest namespace is checked with the version in S13, after the proof (S9).
	if cerr := s.checkShape(c.Context, c.Issuer, s.Namespace); cerr != nil {
		return nil, cerr
	}
	if cerr := s.Trust.Verify(f.Manifest, codes.SyncManifestSignatureInvalid); cerr != nil {
		return nil, cerr
	}
	return pkgv.Verify(f, r.Manifest, s.Namespace, r.Version.String())
}

// accept persists continuity and packages, publishes the snapshot (E1) and runs the follow-ups.
func (s *Sync) accept(l listingCredential, raw []byte, pkgs map[ni.Digest]*pkgv.Package, fresh []*pkgv.Package) *codes.Error {
	var prevKnown map[string]lifecycle.Record
	if s.accepted != nil {
		prevKnown = s.accepted.Known
	}
	for _, p := range fresh {
		if err := s.Packages.Put(p.Digest, p.Raw); err != nil {
			return codes.New(codes.SyncUnreachable, "storing package: %v", err)
		}
	}
	acc := &lifecycle.Accepted{Sequence: l.Subject.Sequence, Digest: ni.Sum(raw), Raw: raw, Known: lifecycle.Merge(prevKnown, l.Subject)}
	doc, at := s.Trust.Cache()
	if err := s.Continuity.Save(Continuity{Accepted: acc, DID: doc, ResolvedAt: at}); err != nil {
		return codes.New(codes.SyncUnreachable, "persisting continuity: %v", err)
	}
	// Packages kept for agreements stay usable after their version leaves the listing; GC decides what goes.
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
	if revs := lifecycle.NewRevocations(prevKnown, l.Subject); len(revs) > 0 {
		s.Agreements.OnRevocations(revs, snap)
	}
	for _, p := range fresh {
		if r, ok := lifecycle.Find(l.Subject.Versions, semver.MustParse(p.Manifest.Version)); ok {
			s.Audit.OnNewVersion(r, snap)
		}
	}
	logx.Default(s.Logger).Info("hub listing accepted", slog.String("namespace", s.Namespace), slog.Uint64("sequence", l.Subject.Sequence), slog.Int("versions", len(l.Subject.Versions)), slog.Int("fresh_packages", len(fresh)))
	return nil
}

// snapshot builds the immutable snapshot of a listing.
func snapshot(l listingCredential, known map[string]lifecycle.Record, pkgs map[ni.Digest]*pkgv.Package) *domain.Snapshot {
	return &domain.Snapshot{
		Namespace: l.Subject.Namespace, Sequence: l.Subject.Sequence, ValidFrom: l.ValidFrom, ValidUntil: *l.ValidUntil,
		Records: l.Subject.Versions, Known: known, Packages: pkgs, Params: domain.HeadParams(l.Subject.Versions, pkgs),
	}
}
