// In-memory implementations of the alexandria-server repositories with per-step fault injection.
// Lets app tests inject failures at each REQ-HUB-004 step and run without SQLite (F8 §8.7).
package testkit

import (
	"fmt"
	"sync"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Faults injects errors into the doubles; a nil field means no failure.
type Faults struct {
	Register      error
	BlobPut       error
	AppendRelease error
	ListingPut    error
}

// MemDB is a shared in-memory store behind all server repository doubles.
type MemDB struct {
	mu          sync.Mutex
	drafts      map[string]domain.Draft
	candidates  map[string]domain.Candidate
	decisions   map[string]domain.Candidate
	releases    []domain.Release
	revocations map[string]domain.Revocation
	listings    []app.StoredListing
	events      []domain.Event
	blobs       map[string][]byte
	seq         int
	Faults      Faults
}

// NewMemDB returns an empty store.
func NewMemDB() *MemDB {
	return &MemDB{drafts: map[string]domain.Draft{}, candidates: map[string]domain.Candidate{}, decisions: map[string]domain.Candidate{},
		revocations: map[string]domain.Revocation{}, blobs: map[string][]byte{}}
}

// MemDrafts adapts MemDB to DraftRepo.
type MemDrafts struct{ *MemDB }

// Create stores a draft at revision 1.
func (m MemDrafts) Create(doc []byte) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	d := domain.Draft{ID: fmt.Sprintf("d%d", m.seq), Rev: 1, Doc: append([]byte(nil), doc...)}
	d.ETag = fmt.Sprintf(`"%s-%d"`, d.ID, d.Rev)
	m.drafts[d.ID] = d
	return d, nil
}

// Get returns a draft.
func (m MemDrafts) Get(id string) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drafts[id]
	if !ok {
		return domain.Draft{}, app.ErrNotFound
	}
	return d, nil
}

// Replace bumps the revision when ifMatch is current.
func (m MemDrafts) Replace(id, ifMatch string, doc []byte) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drafts[id]
	if !ok {
		return domain.Draft{}, app.ErrNotFound
	}
	if d.ETag != ifMatch {
		return domain.Draft{}, app.ErrPreconditionFailed
	}
	d.Rev++
	d.Doc = append([]byte(nil), doc...)
	d.ETag = fmt.Sprintf(`"%s-%d"`, d.ID, d.Rev)
	m.drafts[id] = d
	return d, nil
}

// MemCandidates adapts MemDB to CandidateRepo.
type MemCandidates struct{ *MemDB }

// Insert stores a frozen candidate.
func (m MemCandidates) Insert(c domain.Candidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.candidates[c.ID] = c
	return nil
}

// Get returns a candidate with its latest decision applied.
func (m MemCandidates) Get(id string) (domain.Candidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[id]
	if !ok {
		return domain.Candidate{}, app.ErrNotFound
	}
	if d, ok := m.decisions[id]; ok {
		c.State, c.Comment = d.State, d.Comment
	}
	return c, nil
}

// Reject appends a rejection decision.
func (m MemCandidates) Reject(id, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.decisions[id]; ok {
		return app.ErrInvalidState
	}
	m.decisions[id] = domain.Candidate{State: domain.Rejected, Comment: comment}
	return nil
}

// MemReleases adapts MemDB to ReleaseStore.
type MemReleases struct{ *MemDB }

// AppendRelease appends a release and approves its candidate atomically.
func (m MemReleases) AppendRelease(r domain.Release, from string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.AppendRelease != nil {
		return m.Faults.AppendRelease
	}
	for _, x := range m.releases {
		if semver.Compare(x.Record.Version, r.Record.Version) == 0 {
			return app.ErrInvalidState
		}
	}
	m.releases = append(m.releases, r)
	m.decisions[from] = domain.Candidate{State: domain.Approved}
	return nil
}

// AppendRevocation appends a revocation once per version.
func (m MemReleases) AppendRevocation(rv domain.Revocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.revocations[rv.Version.String()]; ok {
		return app.ErrInvalidState
	}
	m.revocations[rv.Version.String()] = rv
	return nil
}

// Records projects releases and revocations into records.
func (m MemReleases) Records() ([]lifecycle.Record, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []lifecycle.Record
	for _, r := range m.releases {
		rec := r.Record
		if rv, ok := m.revocations[rec.Version.String()]; ok {
			rf := rv.RevokedFrom
			rec.RevokedFrom = &rf
		}
		out = append(out, rec)
	}
	return lifecycle.SortAsc(out), nil
}

// Release returns the release of v.
func (m MemReleases) Release(v semver.Version) (domain.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.releases {
		if semver.Compare(r.Record.Version, v) == 0 {
			return r, nil
		}
	}
	return domain.Release{}, app.ErrNotFound
}

// ByManifest returns the release whose manifest has digest d.
func (m MemReleases) ByManifest(d ni.Digest) (domain.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.releases {
		if r.Record.Manifest == d {
			return r, nil
		}
	}
	return domain.Release{}, app.ErrNotFound
}

// Releases returns every release.
func (m MemReleases) Releases() ([]domain.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Release(nil), m.releases...), nil
}

// HasBlob reports whether a release references the blob.
func (m MemReleases) HasBlob(hex string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.releases {
		for _, e := range r.Entries {
			if e.Digest.Hex() == hex {
				return true, nil
			}
		}
	}
	return false, nil
}

// MemListings adapts MemDB to ListingStore.
type MemListings struct{ *MemDB }

// Put stores a listing with a unique sequence.
func (m MemListings) Put(l app.StoredListing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.ListingPut != nil {
		return m.Faults.ListingPut
	}
	for _, x := range m.listings {
		if x.Sequence == l.Sequence {
			return app.ErrSequenceTaken
		}
	}
	m.listings = append(m.listings, l)
	return nil
}

// Latest returns the last stored listing.
func (m MemListings) Latest() (app.StoredListing, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.listings) == 0 {
		return app.StoredListing{}, false, nil
	}
	return m.listings[len(m.listings)-1], true, nil
}

// All returns every issued listing in order.
func (m MemListings) All() []app.StoredListing {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]app.StoredListing(nil), m.listings...)
}

// MemBlobs adapts MemDB to BlobStore.
type MemBlobs struct{ *MemDB }

// Put stores a blob by hash.
func (m MemBlobs) Put(hex string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.BlobPut != nil {
		return m.Faults.BlobPut
	}
	m.blobs[hex] = append([]byte(nil), b...)
	return nil
}

// Get returns a blob by hash.
func (m MemBlobs) Get(hex string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[hex]
	if !ok {
		return nil, app.ErrNotFound
	}
	return b, nil
}

// MemEvents adapts MemDB to EventLog.
type MemEvents struct{ *MemDB }

// Append assigns the next id.
func (m MemEvents) Append(e domain.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = int64(len(m.events) + 1)
	m.events = append(m.events, e)
	return e.ID, nil
}

// After returns events with id greater than after.
func (m MemEvents) After(after int64, limit int) ([]domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []domain.Event
	for _, e := range m.events {
		if e.ID > after && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

// FailingLog wraps a transparency log and fails when Faults.Register is set.
type FailingLog struct {
	Inner app.TransparencyLog
	DB    *MemDB
}

// Register delegates unless a failure is injected.
func (f FailingLog) Register(d ni.Digest) ([]byte, error) {
	if f.DB.Faults.Register != nil {
		return nil, f.DB.Faults.Register
	}
	return f.Inner.Register(d)
}
