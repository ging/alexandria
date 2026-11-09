// Package app implements the listing use case: issues signed listings on date changes and at most every ttl/2, signs version records, serves the DID document.
// Records and listings come from the same projection, so a record never diverges from its listing entry (REQ-HUB-008..010, REQ-HUB-015, REQ-HUB-021).
package app

import (
	"bytes"
	"encoding/json"
	"sync"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/domain"
)

// Listing implements /v1/versions*, the scheduler pass and /.well-known/did.json.
type Listing struct {
	d  Deps
	mu *sync.Mutex
}

// Issue emits a new listing at t, if any version exists.
func (l *Listing) Issue(t timex.Instant) (StoredListing, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.issue(t)
}

// issue builds, signs and stores the next listing; the caller holds the lock.
func (l *Listing) issue(t timex.Instant) (StoredListing, error) {
	recs, err := l.d.Releases.Records()
	if err != nil || len(recs) == 0 {
		return StoredListing{}, err
	}
	params, space, err := headParams(l.d)
	if err != nil {
		return StoredListing{}, err
	}
	last, ok, err := l.d.Listings.Latest()
	if err != nil {
		return StoredListing{}, err
	}
	seq := uint64(1)
	if ok {
		seq = last.Sequence + 1
	}
	lst := lifecycle.Build(space, recs, t, params.Freshness.TTL, seq)
	raw, err := l.envelope(lst)
	if err != nil {
		return StoredListing{}, err
	}
	s := StoredListing{Sequence: seq, Raw: raw, Listing: lst}
	if err := l.d.Listings.Put(s); err != nil {
		return StoredListing{}, err
	}
	return s, emit(l.d, domain.EvListingIssued, "vhub", lst.IssuedAt.String())
}

// IssueIfStale emits a listing when none exists or the last one is at least ttl/2 old.
func (l *Listing) IssueIfStale(t timex.Instant) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	last, ok, err := l.d.Listings.Latest()
	if err != nil {
		return err
	}
	if ok {
		ttl := last.Listing.ExpiresAt.Sub(last.Listing.IssuedAt)
		if t < last.Listing.IssuedAt.Add(ttl/2) {
			return nil
		}
	}
	_, err = l.issue(t)
	return err
}

// Latest returns the current listing, issuing a fresh one first if the last is stale.
func (l *Listing) Latest(p domain.Principal) ([]byte, error) {
	if err := authorize(p, domain.VersionsList); err != nil {
		return nil, err
	}
	if err := l.IssueIfStale(l.d.Clock.Now()); err != nil {
		return nil, err
	}
	last, ok, err := l.d.Listings.Latest()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, codes.New(codes.APINotFound, "no version has been published yet")
	}
	return last.Raw, nil
}

// Record returns the signed record of a published version.
func (l *Listing) Record(p domain.Principal, version string) ([]byte, error) {
	if err := authorize(p, domain.VersionsGet); err != nil {
		return nil, err
	}
	v, err := semver.Parse(version)
	if err != nil {
		return nil, codes.New(codes.APINotFound, "version %s not found", version)
	}
	return l.signRecord(v)
}

// signRecord signs the current projection of version v.
func (l *Listing) signRecord(v semver.Version) ([]byte, error) {
	recs, err := l.d.Releases.Records()
	if err != nil {
		return nil, err
	}
	rec, ok := lifecycle.Find(recs, v)
	if !ok {
		return nil, codes.New(codes.APINotFound, "version %s not found", v)
	}
	return l.signDoc(rec, l.d.Clock.Now())
}

// signDoc builds and signs a version record document issued at t.
func (l *Listing) signDoc(rec lifecycle.Record, t timex.Instant) ([]byte, error) {
	_, space, err := headParams(l.d)
	if err != nil {
		return nil, err
	}
	return l.envelope(lifecycle.VersionDoc{Type: lifecycle.RecordType, Space: space, IssuedAt: t, Record: rec})
}

// envelope serializes {signed, signature} with signed in JCS form, signing JCS(signed).
func (l *Listing) envelope(signed any) ([]byte, error) {
	return Envelope(l.d.Signer, signed)
}

// Envelope signs a document with the hub key and serializes it deterministically.
func Envelope(s Signer, signed any) ([]byte, error) {
	msg, err := sign.SigningInput(signed)
	if err != nil {
		return nil, err
	}
	sig, err := s.Sign(msg)
	if err != nil {
		return nil, err
	}
	sb, _ := json.Marshal(sig)
	var buf bytes.Buffer
	buf.WriteString(`{"signed":`)
	buf.Write(msg)
	buf.WriteString(`,"signature":`)
	buf.Write(sb)
	buf.WriteString(`}`)
	return buf.Bytes(), nil
}

// DIDDocument returns the hub DID document built from the published keys.
func (l *Listing) DIDDocument() did.Document {
	return did.NewDocument(l.d.DID, l.d.Signer.PublishedKeys())
}
