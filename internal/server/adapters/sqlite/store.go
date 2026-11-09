// SQLite adapter (modernc.org/sqlite, no cgo) for drafts, candidates, releases, revocations, listings and events.
// Immutable tables only accept INSERT and SELECT: triggers abort any UPDATE or DELETE on them (DP-2, REQ-HUB-013, REQ-SYS-005, REQ-SYS-006, REQ-HUB-020).
package sqlite

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"

	_ "modernc.org/sqlite"
)

// ImmutableTables are append-only (REQ-HUB-013).
var ImmutableTables = []string{"candidates", "candidate_decisions", "releases", "release_files", "revocations", "listings", "events"}

const schema = `
CREATE TABLE IF NOT EXISTS drafts(id TEXT PRIMARY KEY, rev INTEGER NOT NULL, doc BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS candidates(id TEXT PRIMARY KEY, data BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS candidate_decisions(candidate_id TEXT PRIMARY KEY, state TEXT NOT NULL, comment TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS releases(version TEXT PRIMARY KEY, manifest TEXT NOT NULL UNIQUE, data BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS release_files(version TEXT NOT NULL, path TEXT NOT NULL, hex TEXT NOT NULL, PRIMARY KEY(version, path));
CREATE TABLE IF NOT EXISTS revocations(version TEXT PRIMARY KEY, data BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS listings(sequence INTEGER PRIMARY KEY, raw BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT, ts TEXT NOT NULL, type TEXT NOT NULL, principal TEXT NOT NULL, subject TEXT NOT NULL);
`

// Store is a single SQLite database implementing every server repository port.
type Store struct{ db *sql.DB }

// Open opens or creates the database and installs the schema and immutability triggers.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(schema); err != nil {
		return nil, err
	}
	for _, t := range ImmutableTables {
		for _, op := range []string{"UPDATE", "DELETE"} {
			q := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_no_%s BEFORE %s ON %s BEGIN SELECT RAISE(ABORT, '%s is append-only'); END;`,
				t, strings.ToLower(op), op, t, t)
			if _, err := db.Exec(q); err != nil {
				return nil, err
			}
		}
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the handle for inspection tests.
func (s *Store) DB() *sql.DB { return s.db }

// Drafts returns the DraftRepo view.
func (s *Store) Drafts() app.DraftRepo { return drafts{s} }

// Candidates returns the CandidateRepo view.
func (s *Store) Candidates() app.CandidateRepo { return candidates{s} }

// Releases returns the ReleaseStore view.
func (s *Store) Releases() app.ReleaseStore { return releases{s} }

// Listings returns the ListingStore view.
func (s *Store) Listings() app.ListingStore { return listings{s} }

// Events returns the EventLog view.
func (s *Store) Events() app.EventLog { return events{s} }

// drafts implements DraftRepo; drafts are the only mutable rows.
type drafts struct{ *Store }

// etag renders the ETag of a draft revision.
func etag(id string, rev int) string { return fmt.Sprintf(`"%s-%d"`, id, rev) }

// Create inserts a draft at revision 1.
func (s drafts) Create(doc []byte) (domain.Draft, error) {
	id := newID()
	if _, err := s.db.Exec(`INSERT INTO drafts(id, rev, doc) VALUES(?, 1, ?)`, id, doc); err != nil {
		return domain.Draft{}, err
	}
	return domain.Draft{ID: id, Rev: 1, Doc: doc, ETag: etag(id, 1)}, nil
}

// Get reads a draft.
func (s drafts) Get(id string) (domain.Draft, error) {
	d := domain.Draft{ID: id}
	err := s.db.QueryRow(`SELECT rev, doc FROM drafts WHERE id = ?`, id).Scan(&d.Rev, &d.Doc)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Draft{}, app.ErrNotFound
	}
	d.ETag = etag(id, d.Rev)
	return d, err
}

// Replace updates a draft only if its revision still matches ifMatch.
func (s drafts) Replace(id, ifMatch string, doc []byte) (domain.Draft, error) {
	cur, err := s.Get(id)
	if err != nil {
		return domain.Draft{}, err
	}
	if cur.ETag != ifMatch {
		return domain.Draft{}, app.ErrPreconditionFailed
	}
	res, err := s.db.Exec(`UPDATE drafts SET rev = rev + 1, doc = ? WHERE id = ? AND rev = ?`, doc, id, cur.Rev)
	if err != nil {
		return domain.Draft{}, err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return domain.Draft{}, app.ErrPreconditionFailed
	}
	return domain.Draft{ID: id, Rev: cur.Rev + 1, Doc: doc, ETag: etag(id, cur.Rev+1)}, nil
}

// candidates implements CandidateRepo; decisions are separate append-only rows.
type candidates struct{ *Store }

// storedCandidate keeps the candidate together with its frozen package.
type storedCandidate struct {
	Candidate domain.Candidate `json:"candidate"`
	Package   domain.Compiled  `json:"package"`
}

// Insert stores a frozen candidate.
func (s candidates) Insert(c domain.Candidate) error {
	b, err := json.Marshal(storedCandidate{c, c.Package})
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO candidates(id, data) VALUES(?, ?)`, c.ID, b)
	return err
}

// Get reads a candidate and applies its decision, if any.
func (s candidates) Get(id string) (domain.Candidate, error) {
	var b []byte
	err := s.db.QueryRow(`SELECT data FROM candidates WHERE id = ?`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Candidate{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Candidate{}, err
	}
	var sc storedCandidate
	if err := json.Unmarshal(b, &sc); err != nil {
		return domain.Candidate{}, err
	}
	c := sc.Candidate
	c.Package = sc.Package
	var state, comment string
	switch err := s.db.QueryRow(`SELECT state, comment FROM candidate_decisions WHERE candidate_id = ?`, id).Scan(&state, &comment); {
	case err == nil:
		c.State, c.Comment = domain.CandidateState(state), comment
	case !errors.Is(err, sql.ErrNoRows):
		return domain.Candidate{}, err
	}
	return c, nil
}

// Reject appends a rejection decision.
func (s candidates) Reject(id, comment string) error {
	_, err := s.db.Exec(`INSERT INTO candidate_decisions(candidate_id, state, comment) VALUES(?, ?, ?)`, id, domain.Rejected, comment)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return app.ErrInvalidState
	}
	return err
}

// releases implements ReleaseStore.
type releases struct{ *Store }

// AppendRelease inserts the release, its files and the approval decision in one transaction.
func (s releases) AppendRelease(r domain.Release, from string) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.Exec(`INSERT INTO releases(version, manifest, data) VALUES(?, ?, ?)`, r.Record.Version.String(), string(r.Record.Manifest), b); err != nil {
		return err
	}
	for _, e := range r.Entries {
		if _, err := tx.Exec(`INSERT INTO release_files(version, path, hex) VALUES(?, ?, ?)`, r.Record.Version.String(), e.Path, e.Digest.Hex()); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO candidate_decisions(candidate_id, state, comment) VALUES(?, ?, '')`, from, domain.Approved); err != nil {
		return err
	}
	return tx.Commit()
}

// AppendRevocation inserts the signed revocation of a version.
func (s releases) AppendRevocation(rv domain.Revocation) error {
	b, err := json.Marshal(rv)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`INSERT INTO revocations(version, data) VALUES(?, ?)`, rv.Version.String(), b)
	return err
}

// Releases reads every release.
func (s releases) Releases() ([]domain.Release, error) {
	rows, err := s.db.Query(`SELECT data FROM releases`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Release
	for rows.Next() {
		var b []byte
		var r domain.Release
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &r); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// Records projects releases with their revocations.
func (s releases) Records() ([]lifecycle.Record, error) {
	rels, err := s.Releases()
	if err != nil {
		return nil, err
	}
	revs := map[string]timex.Instant{}
	rows, err := s.db.Query(`SELECT data FROM revocations`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b []byte
		var rv domain.Revocation
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, &rv); err != nil {
			return nil, err
		}
		revs[rv.Version.String()] = rv.RevokedFrom
	}
	var out []lifecycle.Record
	for _, r := range rels {
		rec := r.Record
		if rf, ok := revs[rec.Version.String()]; ok {
			rec.RevokedFrom = &rf
		}
		out = append(out, rec)
	}
	return lifecycle.SortAsc(out), rows.Err()
}

// one reads a single release with a query and argument.
func (s releases) one(q string, arg any) (domain.Release, error) {
	var b []byte
	err := s.db.QueryRow(q, arg).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Release{}, app.ErrNotFound
	}
	if err != nil {
		return domain.Release{}, err
	}
	var r domain.Release
	return r, json.Unmarshal(b, &r)
}

// Release reads the release of v.
func (s releases) Release(v semver.Version) (domain.Release, error) {
	return s.one(`SELECT data FROM releases WHERE version = ?`, v.String())
}

// ByManifest reads the release with manifest digest d.
func (s releases) ByManifest(d ni.Digest) (domain.Release, error) {
	return s.one(`SELECT data FROM releases WHERE manifest = ?`, string(d))
}

// HasBlob reports whether some release references the blob.
func (s releases) HasBlob(hex string) (bool, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM release_files WHERE hex = ?`, hex).Scan(&n)
	return n > 0, err
}

// listings implements ListingStore; the primary key enforces sequence uniqueness.
type listings struct{ *Store }

// Put inserts an issued listing.
func (s listings) Put(l app.StoredListing) error {
	_, err := s.db.Exec(`INSERT INTO listings(sequence, raw) VALUES(?, ?)`, l.Sequence, l.Raw)
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return app.ErrSequenceTaken
	}
	return err
}

// Latest reads the listing with the highest sequence.
func (s listings) Latest() (app.StoredListing, bool, error) {
	var l app.StoredListing
	err := s.db.QueryRow(`SELECT sequence, raw FROM listings ORDER BY sequence DESC LIMIT 1`).Scan(&l.Sequence, &l.Raw)
	if errors.Is(err, sql.ErrNoRows) {
		return app.StoredListing{}, false, nil
	}
	if err != nil {
		return app.StoredListing{}, false, err
	}
	var env sign.Envelope[lifecycle.Listing]
	if err := json.Unmarshal(l.Raw, &env); err != nil {
		return app.StoredListing{}, false, err
	}
	l.Listing = env.Signed
	return l, true, nil
}

// events implements EventLog with AUTOINCREMENT ids.
type events struct{ *Store }

// Append inserts an event and returns its id.
func (s events) Append(e domain.Event) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO events(ts, type, principal, subject) VALUES(?, ?, ?, ?)`, e.TS.String(), e.Type, e.Principal, e.Subject)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// After reads events after an id.
func (s events) After(after int64, limit int) ([]domain.Event, error) {
	rows, err := s.db.Query(`SELECT id, ts, type, principal, subject FROM events WHERE id > ? ORDER BY id LIMIT ?`, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Event
	for rows.Next() {
		var e domain.Event
		var ts string
		if err := rows.Scan(&e.ID, &ts, &e.Type, &e.Principal, &e.Subject); err != nil {
			return nil, err
		}
		e.TS, _ = timex.ParseInstant(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}
