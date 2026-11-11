// Package app implements the Listing use case and the signing-key gate: per-namespace signed listings and stored version records.
// The hub signs only after resolving its own DID and finding the active key in assertionMethod (REQ-HUB-008..010, REQ-HUB-015, REQ-HUB-021, REQ-HUB-031).
package app

import (
	"sync"
	"sync/atomic"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/domain"
)

// KeyGate refuses to sign until CheckSigningKey has seen the active key in the hub DID document.
type KeyGate struct {
	d         Deps
	published atomic.Bool
	checked   atomic.Bool
}

// CheckSigningKey resolves the hub DID and records whether the active key is an assertion method.
func (k *KeyGate) CheckSigningKey() bool {
	doc, err := k.d.Resolver.Resolve(k.d.DID)
	ok := err == nil && doc.HasKid(k.d.Signer.VerificationMethod())
	was, first := k.published.Swap(ok), !k.checked.Swap(true)
	if !ok && (was || first) {
		_ = k.d.emit(nil, domain.EvSignerKeyUnpublished, "", k.d.Signer.VerificationMethod())
	}
	return ok
}

// Sign signs an unsecured credential, or fails with SIGNER_KEY_NOT_PUBLISHED.
func (k *KeyGate) Sign(unsecured []byte, created timex.Instant) ([]byte, error) {
	if !k.published.Load() {
		return nil, codes.New(codes.SignerKeyNotPublished, "%s is not an assertionMethod of %s", k.d.Signer.VerificationMethod(), k.d.DID)
	}
	return k.d.Signer.Sign(unsecured, created)
}

// Listing implements .../versions*, and the scheduler pass.
type Listing struct {
	d    Deps
	keys *KeyGate
	mu   *sync.Mutex
}

// Issue emits a new listing of ns at t, if the namespace has any version.
func (l *Listing) Issue(ns string, t timex.Instant) (StoredListing, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.issue(ns, t)
}

// issue builds, signs and stores the next listing; the caller holds the lock.
func (l *Listing) issue(ns string, t timex.Instant) (StoredListing, error) {
	recs, _, err := l.d.records(ns)
	if err != nil || len(recs) == 0 {
		return StoredListing{}, err
	}
	params, err := l.d.headParams(ns)
	if err != nil {
		return StoredListing{}, err
	}
	last, ok, err := l.d.Store.LatestListing(ns)
	if err != nil {
		return StoredListing{}, err
	}
	seq := uint64(1)
	if ok {
		seq = last.Sequence + 1
	}
	subject := lifecycle.Build(l.d.nsIRI(ns), recs, t, seq)
	until := t.Add(params.Freshness.TTL)
	raw, err := l.keys.Sign(l.d.unsigned(vc.TypeVersionListing, t, &until, subject), t)
	if err != nil {
		return StoredListing{}, err
	}
	s := StoredListing{Sequence: seq, Raw: raw, IssuedAt: t, ValidUntil: until, Subject: subject}
	if err := l.d.Store.PutListing(ns, s); err != nil {
		return StoredListing{}, err
	}
	return s, l.d.emit(nil, domain.EvListingIssued, ns, subject.ID)
}

// IssueIfStale emits a listing when none exists or the last one is at least ttl/2 old (REQ-HUB-008).
func (l *Listing) IssueIfStale(ns string, t timex.Instant) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	last, ok, err := l.d.Store.LatestListing(ns)
	if err != nil {
		return err
	}
	if ok && t < last.IssuedAt.Add(last.ValidUntil.Sub(last.IssuedAt)/2) {
		return nil
	}
	_, err = l.issue(ns, t)
	return err
}

// Tick reissues stale listings of every namespace; the scheduler calls it.
func (l *Listing) Tick(t timex.Instant) error {
	nss, err := l.d.Store.ListNamespaces()
	if err != nil {
		return err
	}
	for _, n := range nss {
		if err := l.IssueIfStale(n.Slug, t); err != nil {
			return err
		}
	}
	return nil
}

// Latest returns the current listing of ns (public), issuing a fresh one first if the last is stale.
func (l *Listing) Latest(ns string) ([]byte, error) {
	if _, err := l.d.namespace(ns); err != nil {
		return nil, err
	}
	if err := l.IssueIfStale(ns, l.d.Clock.Now()); err != nil {
		if _, coded := codes.As(err); !coded {
			return nil, err
		}
	}
	last, ok, err := l.d.Store.LatestListing(ns)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, codes.New(codes.APINotFound, "namespace %s has no published version", ns)
	}
	return last.Raw, nil
}

// Record returns the stored signed record of a published version (public).
func (l *Listing) Record(ns, version string) ([]byte, error) {
	if _, err := l.d.namespace(ns); err != nil {
		return nil, err
	}
	v, err := semver.Parse(version)
	if err != nil {
		return nil, codes.New(codes.APINotFound, "version %s not found", version)
	}
	_, rels, err := l.d.records(ns)
	if err != nil {
		return nil, err
	}
	for _, r := range rels {
		if semver.Compare(r.Record.Version, v) == 0 {
			return r.SignedRec, nil
		}
	}
	return nil, codes.New(codes.APINotFound, "version %s not found", version)
}
