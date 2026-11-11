// Package testkit provides an in-memory implementation of every alexandria-server persistence port with fault injection.
// Lets app tests inject failures at each REQ-HUB-004 step and run without SQLite (F8 §8.7).
package testkit

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Faults injects errors into the doubles; a nil field means no failure.
type Faults struct {
	BlobPut       error
	AppendRelease error
	ListingPut    error
}

// MemDB is the in-memory store behind app.Store and app.BlobStore.
type MemDB struct {
	mu          sync.Mutex
	namespaces  map[string]domain.Namespace
	stewards    map[string][]domain.Steward
	vocabs      map[string]domain.Vocabulary
	drafts      map[string]domain.Draft
	candidates  map[string]domain.Candidate
	releases    []domain.Release
	revocations map[string]domain.Revocation
	listings    map[string][]app.StoredListing
	events      []domain.Event
	blobs       map[string][]byte
	seq         int
	Faults      Faults
}

var _ app.Store = (*MemDB)(nil)

// NewMemDB returns an empty store.
func NewMemDB() *MemDB {
	return &MemDB{
		namespaces: map[string]domain.Namespace{}, stewards: map[string][]domain.Steward{}, vocabs: map[string]domain.Vocabulary{},
		drafts: map[string]domain.Draft{}, candidates: map[string]domain.Candidate{}, revocations: map[string]domain.Revocation{},
		listings: map[string][]app.StoredListing{}, blobs: map[string][]byte{},
	}
}

// key joins a namespace and an id.
func key(ns, id string) string { return ns + "/" + id }

// CreateNamespace stores a namespace once.
func (m *MemDB) CreateNamespace(n domain.Namespace) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.namespaces[n.Slug]; ok {
		return app.ErrExists
	}
	m.namespaces[n.Slug] = n
	return nil
}

// GetNamespace returns a namespace.
func (m *MemDB) GetNamespace(slug string) (domain.Namespace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n, ok := m.namespaces[slug]
	if !ok {
		return domain.Namespace{}, app.ErrNotFound
	}
	return n, nil
}

// ListNamespaces returns every namespace by slug.
func (m *MemDB) ListNamespaces() ([]domain.Namespace, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Namespace{}
	for _, n := range m.namespaces {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	return out, nil
}

// AddSteward stores a steward; (iss, sub) is unique within a namespace.
func (m *MemDB) AddSteward(ns string, s domain.Steward) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.stewards[ns] {
		if x.Iss == s.Iss && x.Sub == s.Sub {
			return app.ErrExists
		}
	}
	m.stewards[ns] = append(m.stewards[ns], s)
	return nil
}

// RemoveSteward withdraws a steward.
func (m *MemDB) RemoveSteward(ns, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i, x := range m.stewards[ns] {
		if x.ID == id {
			m.stewards[ns] = append(m.stewards[ns][:i:i], m.stewards[ns][i+1:]...)
			return nil
		}
	}
	return app.ErrNotFound
}

// IsSteward reports membership of (iss, sub) in ns.
func (m *MemDB) IsSteward(ns, iss, sub string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, x := range m.stewards[ns] {
		if x.Iss == iss && x.Sub == sub {
			return true, nil
		}
	}
	return false, nil
}

// ListStewards returns the stewards of ns.
func (m *MemDB) ListStewards(ns string) ([]domain.Steward, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]domain.Steward{}, m.stewards[ns]...), nil
}

// CreateVocabulary stores a vocabulary once per namespace.
func (m *MemDB) CreateVocabulary(ns string, v domain.Vocabulary) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.vocabs[key(ns, v.Name)]; ok {
		return app.ErrExists
	}
	v.Revisions = []domain.Revision{}
	m.vocabs[key(ns, v.Name)] = v
	return nil
}

// GetVocabulary returns a vocabulary with its revisions.
func (m *MemDB) GetVocabulary(ns, name string) (domain.Vocabulary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.vocabs[key(ns, name)]
	if !ok {
		return domain.Vocabulary{}, app.ErrNotFound
	}
	v.Revisions = append([]domain.Revision{}, v.Revisions...)
	return v, nil
}

// AppendRevision appends a revision.
func (m *MemDB) AppendRevision(ns, name string, r domain.Revision) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	v, ok := m.vocabs[key(ns, name)]
	if !ok {
		return app.ErrNotFound
	}
	v.Revisions = append(v.Revisions, r)
	m.vocabs[key(ns, name)] = v
	return nil
}

// ListVocabularies returns the vocabularies of ns by name.
func (m *MemDB) ListVocabularies(ns string) ([]domain.Vocabulary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Vocabulary{}
	for k, v := range m.vocabs {
		if strings.HasPrefix(k, ns+"/") {
			out = append(out, v)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// CreateDraft stores a draft at revision 1.
func (m *MemDB) CreateDraft(ns string, doc []byte) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.seq++
	d := domain.Draft{ID: fmt.Sprintf("d%d", m.seq), Namespace: ns, Rev: 1, Doc: append([]byte(nil), doc...)}
	d.ETag = fmt.Sprintf(`"%s-%d"`, d.ID, d.Rev)
	m.drafts[key(ns, d.ID)] = d
	return d, nil
}

// GetDraft returns a draft of ns.
func (m *MemDB) GetDraft(ns, id string) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drafts[key(ns, id)]
	if !ok {
		return domain.Draft{}, app.ErrNotFound
	}
	return d, nil
}

// ReplaceDraft replaces a draft when ifMatch is its ETag.
func (m *MemDB) ReplaceDraft(ns, id, ifMatch string, doc []byte) (domain.Draft, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	d, ok := m.drafts[key(ns, id)]
	if !ok {
		return domain.Draft{}, app.ErrNotFound
	}
	if d.ETag != ifMatch {
		return domain.Draft{}, app.ErrPreconditionFailed
	}
	d.Rev++
	d.Doc = append([]byte(nil), doc...)
	d.ETag = fmt.Sprintf(`"%s-%d"`, d.ID, d.Rev)
	m.drafts[key(ns, id)] = d
	return d, nil
}

// InsertCandidate stores a candidate.
func (m *MemDB) InsertCandidate(c domain.Candidate) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.candidates[key(c.Namespace, c.ID)] = c
	return nil
}

// GetCandidate returns a candidate of ns.
func (m *MemDB) GetCandidate(ns, id string) (domain.Candidate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[key(ns, id)]
	if !ok {
		return domain.Candidate{}, app.ErrNotFound
	}
	return c, nil
}

// RejectCandidate moves an open candidate to rejected.
func (m *MemDB) RejectCandidate(ns, id, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.candidates[key(ns, id)]
	if !ok || c.State != domain.Open {
		return app.ErrInvalidState
	}
	c.State, c.Comment = domain.Rejected, comment
	m.candidates[key(ns, id)] = c
	return nil
}

// AppendRelease stores a release and approves its candidate atomically.
func (m *MemDB) AppendRelease(r domain.Release, from string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.AppendRelease != nil {
		return m.Faults.AppendRelease
	}
	c, ok := m.candidates[key(r.Namespace, from)]
	if !ok || c.State != domain.Open {
		return app.ErrInvalidState
	}
	c.State = domain.Approved
	m.candidates[key(r.Namespace, from)] = c
	m.releases = append(m.releases, r)
	return nil
}

// AppendRevocation stores a revocation once per version.
func (m *MemDB) AppendRevocation(rv domain.Revocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(rv.Namespace, rv.Version.String())
	if _, ok := m.revocations[k]; ok {
		return app.ErrInvalidState
	}
	m.revocations[k] = rv
	return nil
}

// Releases returns the releases of ns with revocations applied.
func (m *MemDB) Releases(ns string) ([]domain.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Release{}
	for _, r := range m.releases {
		if r.Namespace != ns {
			continue
		}
		if rv, ok := m.revocations[key(ns, r.Record.Version.String())]; ok {
			rf := rv.RevokedFrom
			r.Record.RevokedFrom, r.SignedRec = &rf, rv.SignedRec
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return semver.Less(out[i].Record.Version, out[j].Record.Version) })
	return out, nil
}

// ByManifest returns the release whose manifest has digest d.
func (m *MemDB) ByManifest(d ni.Digest) (domain.Release, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range m.releases {
		if r.Record.Manifest == d {
			return r, nil
		}
	}
	return domain.Release{}, app.ErrNotFound
}

// PutListing stores a listing; its sequence must be the next one of ns.
func (m *MemDB) PutListing(ns string, l app.StoredListing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.ListingPut != nil {
		return m.Faults.ListingPut
	}
	for _, x := range m.listings[ns] {
		if x.Sequence == l.Sequence {
			return app.ErrSequenceTaken
		}
	}
	m.listings[ns] = append(m.listings[ns], l)
	return nil
}

// LatestListing returns the last listing of ns.
func (m *MemDB) LatestListing(ns string) (app.StoredListing, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	ls := m.listings[ns]
	if len(ls) == 0 {
		return app.StoredListing{}, false, nil
	}
	return ls[len(ls)-1], true, nil
}

// Listings returns every listing issued for ns, oldest first.
func (m *MemDB) Listings(ns string) []app.StoredListing {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]app.StoredListing{}, m.listings[ns]...)
}

// Append stores an event with the next id.
func (m *MemDB) Append(e domain.Event) (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e.ID = int64(len(m.events) + 1)
	m.events = append(m.events, e)
	return e.ID, nil
}

// After returns up to limit events of scope ns with an id greater than after.
func (m *MemDB) After(ns string, after int64, limit int) ([]domain.Event, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := []domain.Event{}
	for _, e := range m.events {
		if e.ID > after && e.Namespace == ns && len(out) < limit {
			out = append(out, e)
		}
	}
	return out, nil
}

// MemBlobs adapts MemDB to BlobStore.
type MemBlobs struct{ *MemDB }

// Put stores bytes under their hex digest.
func (m MemBlobs) Put(hex string, b []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.Faults.BlobPut != nil {
		return m.Faults.BlobPut
	}
	m.blobs[hex] = append([]byte(nil), b...)
	return nil
}

// Get returns the bytes of a digest.
func (m MemBlobs) Get(hex string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.blobs[hex]
	if !ok {
		return nil, app.ErrNotFound
	}
	return b, nil
}
