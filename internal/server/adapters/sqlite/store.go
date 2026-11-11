// Package sqlite is the SQLite adapter (modernc.org/sqlite, no cgo) implementing every server persistence port, keyed by namespace.
// Immutable tables only accept INSERT and SELECT: triggers abort any UPDATE or DELETE on them (DP-2, REQ-HUB-013, REQ-HUB-020, REQ-HUB-024).
package sqlite

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"

	// Registers the pure-Go "sqlite" driver used by Open.
	_ "modernc.org/sqlite"
)

// bg is the context of every query: the repository ports are synchronous and carry no context.
var bg = context.Background()

// ImmutableTables are append-only (REQ-HUB-013); stewards and drafts are the only mutable tables.
var ImmutableTables = []string{"namespaces", "vocabularies", "vocab_revisions", "candidates", "candidate_decisions", "releases", "revocations", "listings", "events"}

const schema = `
CREATE TABLE IF NOT EXISTS namespaces(slug TEXT PRIMARY KEY, data BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS stewards(ns TEXT NOT NULL, id TEXT NOT NULL, iss TEXT NOT NULL, sub TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, id), UNIQUE(ns, iss, sub));
CREATE TABLE IF NOT EXISTS vocabularies(ns TEXT NOT NULL, name TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, name));
CREATE TABLE IF NOT EXISTS vocab_revisions(ns TEXT NOT NULL, name TEXT NOT NULL, n INTEGER NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, name, n));
CREATE TABLE IF NOT EXISTS drafts(ns TEXT NOT NULL, id TEXT NOT NULL, rev INTEGER NOT NULL, doc BLOB NOT NULL, PRIMARY KEY(ns, id));
CREATE TABLE IF NOT EXISTS candidates(ns TEXT NOT NULL, id TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, id));
CREATE TABLE IF NOT EXISTS candidate_decisions(ns TEXT NOT NULL, id TEXT NOT NULL, state TEXT NOT NULL, comment TEXT NOT NULL, PRIMARY KEY(ns, id));
CREATE TABLE IF NOT EXISTS releases(ns TEXT NOT NULL, version TEXT NOT NULL, manifest TEXT NOT NULL UNIQUE, data BLOB NOT NULL, PRIMARY KEY(ns, version));
CREATE TABLE IF NOT EXISTS revocations(ns TEXT NOT NULL, version TEXT NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, version));
CREATE TABLE IF NOT EXISTS listings(ns TEXT NOT NULL, sequence INTEGER NOT NULL, data BLOB NOT NULL, PRIMARY KEY(ns, sequence));
CREATE TABLE IF NOT EXISTS events(id INTEGER PRIMARY KEY AUTOINCREMENT, ns TEXT NOT NULL, data BLOB NOT NULL);
`

// Store is a single SQLite database implementing app.Store.
type Store struct{ db *sql.DB }

var _ app.Store = (*Store)(nil)

// Open opens or creates the database and installs the schema and immutability triggers.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(bg, schema); err != nil {
		return nil, err
	}
	for _, t := range ImmutableTables {
		for _, op := range []string{"UPDATE", "DELETE"} {
			// #nosec G201 -- identifiers come from the constant ImmutableTables list, never from input.
			q := fmt.Sprintf(`CREATE TRIGGER IF NOT EXISTS %s_no_%s BEFORE %s ON %s BEGIN SELECT RAISE(ABORT, '%s is append-only'); END;`,
				t, strings.ToLower(op), op, t, t)
			if _, err := db.ExecContext(bg, q); err != nil {
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

// unique maps a constraint violation to app.ErrExists.
func unique(err error) error {
	if err != nil && strings.Contains(err.Error(), "UNIQUE") {
		return app.ErrExists
	}
	return err
}

// one scans a single JSON row into v, mapping no rows to app.ErrNotFound.
func one(row *sql.Row, v any) error {
	var b []byte
	if err := row.Scan(&b); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return app.ErrNotFound
		}
		return err
	}
	return json.Unmarshal(b, v)
}

// all scans JSON rows into a slice of T.
func all[T any](rows *sql.Rows, err error) ([]T, error) {
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []T{}
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var v T
		if err := json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// mustJSON serializes a stored value.
func mustJSON(v any) []byte { b, _ := json.Marshal(v); return b }

// CreateNamespace inserts a namespace once.
func (s *Store) CreateNamespace(n domain.Namespace) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO namespaces(slug, data) VALUES(?, ?)`, n.Slug, mustJSON(n))
	if err != nil && strings.Contains(err.Error(), "PRIMARY KEY") {
		return app.ErrExists
	}
	return unique(err)
}

// GetNamespace returns a namespace.
func (s *Store) GetNamespace(slug string) (domain.Namespace, error) {
	var n domain.Namespace
	return n, one(s.db.QueryRowContext(bg, `SELECT data FROM namespaces WHERE slug = ?`, slug), &n)
}

// ListNamespaces returns every namespace by slug.
func (s *Store) ListNamespaces() ([]domain.Namespace, error) {
	return all[domain.Namespace](s.db.QueryContext(bg, `SELECT data FROM namespaces ORDER BY slug`))
}

// AddSteward inserts a steward; (ns, iss, sub) is unique.
func (s *Store) AddSteward(ns string, st domain.Steward) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO stewards(ns, id, iss, sub, data) VALUES(?, ?, ?, ?, ?)`, ns, st.ID, st.Iss, st.Sub, mustJSON(st))
	return unique(err)
}

// RemoveSteward deletes a steward.
func (s *Store) RemoveSteward(ns, id string) error {
	r, err := s.db.ExecContext(bg, `DELETE FROM stewards WHERE ns = ? AND id = ?`, ns, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return app.ErrNotFound
	}
	return nil
}

// IsSteward reports membership of (iss, sub) in ns.
func (s *Store) IsSteward(ns, iss, sub string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(bg, `SELECT COUNT(*) FROM stewards WHERE ns = ? AND iss = ? AND sub = ?`, ns, iss, sub).Scan(&n)
	return n > 0, err
}

// ListStewards returns the stewards of ns.
func (s *Store) ListStewards(ns string) ([]domain.Steward, error) {
	return all[domain.Steward](s.db.QueryContext(bg, `SELECT data FROM stewards WHERE ns = ? ORDER BY rowid`, ns))
}

// CreateVocabulary inserts the immutable metadata of a vocabulary.
func (s *Store) CreateVocabulary(ns string, v domain.Vocabulary) error {
	v.Revisions = nil
	_, err := s.db.ExecContext(bg, `INSERT INTO vocabularies(ns, name, data) VALUES(?, ?, ?)`, ns, v.Name, mustJSON(v))
	if err != nil && strings.Contains(err.Error(), "PRIMARY KEY") {
		return app.ErrExists
	}
	return unique(err)
}

// GetVocabulary returns a vocabulary with its revisions.
func (s *Store) GetVocabulary(ns, name string) (domain.Vocabulary, error) {
	var v domain.Vocabulary
	if err := one(s.db.QueryRowContext(bg, `SELECT data FROM vocabularies WHERE ns = ? AND name = ?`, ns, name), &v); err != nil {
		return v, err
	}
	revs, err := all[domain.Revision](s.db.QueryContext(bg, `SELECT data FROM vocab_revisions WHERE ns = ? AND name = ? ORDER BY n`, ns, name))
	v.Revisions = revs
	return v, err
}

// AppendRevision inserts a revision.
func (s *Store) AppendRevision(ns, name string, r domain.Revision) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO vocab_revisions(ns, name, n, data) VALUES(?, ?, ?, ?)`, ns, name, r.N, mustJSON(r))
	return err
}

// ListVocabularies returns the vocabularies of ns with their revisions.
func (s *Store) ListVocabularies(ns string) ([]domain.Vocabulary, error) {
	vs, err := all[domain.Vocabulary](s.db.QueryContext(bg, `SELECT data FROM vocabularies WHERE ns = ? ORDER BY name`, ns))
	if err != nil {
		return nil, err
	}
	for i := range vs {
		if vs[i], err = s.GetVocabulary(ns, vs[i].Name); err != nil {
			return nil, err
		}
	}
	return vs, nil
}

// etag renders the ETag of a draft revision.
func etag(id string, rev int) string { return fmt.Sprintf(`"%s-%d"`, id, rev) }

// CreateDraft inserts a draft at revision 1.
func (s *Store) CreateDraft(ns string, doc []byte) (domain.Draft, error) {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	d := domain.Draft{ID: hex.EncodeToString(b), Namespace: ns, Rev: 1, Doc: doc}
	d.ETag = etag(d.ID, 1)
	_, err := s.db.ExecContext(bg, `INSERT INTO drafts(ns, id, rev, doc) VALUES(?, ?, 1, ?)`, ns, d.ID, doc)
	return d, err
}

// GetDraft returns a draft of ns.
func (s *Store) GetDraft(ns, id string) (domain.Draft, error) {
	d := domain.Draft{ID: id, Namespace: ns}
	err := s.db.QueryRowContext(bg, `SELECT rev, doc FROM drafts WHERE ns = ? AND id = ?`, ns, id).Scan(&d.Rev, &d.Doc)
	if errors.Is(err, sql.ErrNoRows) {
		return d, app.ErrNotFound
	}
	d.ETag = etag(id, d.Rev)
	return d, err
}

// ReplaceDraft replaces a draft when ifMatch is its current ETag (compare-and-swap on rev).
func (s *Store) ReplaceDraft(ns, id, ifMatch string, doc []byte) (domain.Draft, error) {
	cur, err := s.GetDraft(ns, id)
	if err != nil {
		return cur, err
	}
	if cur.ETag != ifMatch {
		return cur, app.ErrPreconditionFailed
	}
	r, err := s.db.ExecContext(bg, `UPDATE drafts SET rev = rev + 1, doc = ? WHERE ns = ? AND id = ? AND rev = ?`, doc, ns, id, cur.Rev)
	if err != nil {
		return cur, err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return cur, app.ErrPreconditionFailed
	}
	return s.GetDraft(ns, id)
}

// candidateRow is the stored form of a candidate, including its compiled package.
type candidateRow struct {
	domain.Candidate
	Pkg domain.Compiled `json:"package"`
}

// InsertCandidate inserts a frozen candidate.
func (s *Store) InsertCandidate(c domain.Candidate) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO candidates(ns, id, data) VALUES(?, ?, ?)`, c.Namespace, c.ID, mustJSON(candidateRow{c, c.Package}))
	return err
}

// GetCandidate returns a candidate with its decision applied.
func (s *Store) GetCandidate(ns, id string) (domain.Candidate, error) {
	var row candidateRow
	if err := one(s.db.QueryRowContext(bg, `SELECT data FROM candidates WHERE ns = ? AND id = ?`, ns, id), &row); err != nil {
		return domain.Candidate{}, err
	}
	c := row.Candidate
	c.Package = row.Pkg
	var state, comment string
	err := s.db.QueryRowContext(bg, `SELECT state, comment FROM candidate_decisions WHERE ns = ? AND id = ?`, ns, id).Scan(&state, &comment)
	if err == nil {
		c.State, c.Comment = domain.CandidateState(state), comment
	}
	return c, nil
}

// RejectCandidate appends a rejected decision to an open candidate.
func (s *Store) RejectCandidate(ns, id, comment string) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO candidate_decisions(ns, id, state, comment) VALUES(?, ?, 'rejected', ?)`, ns, id, comment)
	if err != nil {
		return app.ErrInvalidState
	}
	return nil
}

// AppendRelease inserts a release and the approval decision in one transaction.
func (s *Store) AppendRelease(r domain.Release, from string) error {
	tx, err := s.db.BeginTx(bg, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if _, err := tx.ExecContext(bg, `INSERT INTO candidate_decisions(ns, id, state, comment) VALUES(?, ?, 'approved', '')`, r.Namespace, from); err != nil {
		return app.ErrInvalidState
	}
	if _, err := tx.ExecContext(bg, `INSERT INTO releases(ns, version, manifest, data) VALUES(?, ?, ?, ?)`, r.Namespace, r.Record.Version.String(), string(r.Record.Manifest), mustJSON(r)); err != nil {
		return err
	}
	return tx.Commit()
}

// AppendRevocation inserts a revocation once per version.
func (s *Store) AppendRevocation(rv domain.Revocation) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO revocations(ns, version, data) VALUES(?, ?, ?)`, rv.Namespace, rv.Version.String(), mustJSON(rv))
	if err != nil {
		return app.ErrInvalidState
	}
	return nil
}

// Releases returns the releases of ns ordered by version, with revocations applied.
func (s *Store) Releases(ns string) ([]domain.Release, error) {
	rels, err := all[domain.Release](s.db.QueryContext(bg, `SELECT data FROM releases WHERE ns = ?`, ns))
	if err != nil {
		return nil, err
	}
	revs, err := all[domain.Revocation](s.db.QueryContext(bg, `SELECT data FROM revocations WHERE ns = ?`, ns))
	if err != nil {
		return nil, err
	}
	for i := range rels {
		for _, rv := range revs {
			if semver.Compare(rv.Version, rels[i].Record.Version) == 0 {
				rf := rv.RevokedFrom
				rels[i].Record.RevokedFrom, rels[i].SignedRec = &rf, rv.SignedRec
			}
		}
	}
	for i := 1; i < len(rels); i++ {
		for j := i; j > 0 && semver.Less(rels[j].Record.Version, rels[j-1].Record.Version); j-- {
			rels[j], rels[j-1] = rels[j-1], rels[j]
		}
	}
	return rels, nil
}

// ByManifest returns the release whose manifest has digest d.
func (s *Store) ByManifest(d ni.Digest) (domain.Release, error) {
	var r domain.Release
	return r, one(s.db.QueryRowContext(bg, `SELECT data FROM releases WHERE manifest = ?`, string(d)), &r)
}

// PutListing inserts a listing; (ns, sequence) is unique.
func (s *Store) PutListing(ns string, l app.StoredListing) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO listings(ns, sequence, data) VALUES(?, ?, ?)`, ns, l.Sequence, mustJSON(l))
	if err != nil && (strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "PRIMARY KEY")) {
		return app.ErrSequenceTaken
	}
	return err
}

// LatestListing returns the last listing of ns.
func (s *Store) LatestListing(ns string) (app.StoredListing, bool, error) {
	var l app.StoredListing
	err := one(s.db.QueryRowContext(bg, `SELECT data FROM listings WHERE ns = ? ORDER BY sequence DESC LIMIT 1`, ns), &l)
	if errors.Is(err, app.ErrNotFound) {
		return l, false, nil
	}
	return l, err == nil, err
}

// Append inserts an event and returns its id.
func (s *Store) Append(e domain.Event) (int64, error) {
	r, err := s.db.ExecContext(bg, `INSERT INTO events(ns, data) VALUES(?, ?)`, e.Namespace, mustJSON(e))
	if err != nil {
		return 0, err
	}
	return r.LastInsertId()
}

// After returns up to limit events of scope ns with an id greater than after.
func (s *Store) After(ns string, after int64, limit int) ([]domain.Event, error) {
	rows, err := s.db.QueryContext(bg, `SELECT id, data FROM events WHERE ns = ? AND id > ? ORDER BY id LIMIT ?`, ns, after, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []domain.Event{}
	for rows.Next() {
		var id int64
		var b []byte
		if err := rows.Scan(&id, &b); err != nil {
			return nil, err
		}
		var e domain.Event
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		e.ID = id
		out = append(out, e)
	}
	return out, rows.Err()
}
