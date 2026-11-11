// Package store_sqlite is the SQLite adapter (modernc.org/sqlite) of the client: continuity, rule set and agreement registry.
// State survives restarts; the rule set is stored whole with its revision (DP-2, REQ-CLI-005, REQ-CLI-031, REQ-SYS-006).
package store_sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"

	// Registers the pure-Go "sqlite" driver used by Open.
	_ "modernc.org/sqlite"
)

// bg is the context of every query.
var bg = context.Background()

const schema = `
CREATE TABLE IF NOT EXISTS kv(k TEXT PRIMARY KEY, v BLOB NOT NULL);
CREATE TABLE IF NOT EXISTS agreements(id TEXT PRIMARY KEY, data BLOB NOT NULL);
`

// Store implements ContinuityStore, RuleStore and AgreementRepo.
type Store struct{ db *sql.DB }

var (
	_ app.ContinuityStore = (*Store)(nil)
	_ app.RuleStore       = Rules{}
	_ app.AgreementRepo   = Agreements{}
)

// Open opens or creates the database.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", "file:"+path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.ExecContext(bg, schema); err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database.
func (s *Store) Close() error { return s.db.Close() }

// get reads a key; ok is false when absent.
func (s *Store) get(k string) ([]byte, bool, error) {
	var b []byte
	err := s.db.QueryRowContext(bg, `SELECT v FROM kv WHERE k = ?`, k).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return b, err == nil, err
}

// put writes a key.
func (s *Store) put(k string, b []byte) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO kv(k, v) VALUES(?, ?) ON CONFLICT(k) DO UPDATE SET v = excluded.v`, k, b)
	return err
}

// Load returns the persisted continuity.
func (s *Store) Load() (app.Continuity, error) {
	var c app.Continuity
	b, ok, err := s.get("continuity")
	if err != nil || !ok {
		return c, err
	}
	return c, json.Unmarshal(b, &c)
}

// Save persists the continuity.
func (s *Store) Save(c app.Continuity) error {
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	return s.put("continuity", b)
}

// Rules is the RuleStore view.
type Rules struct{ S *Store }

// Load returns the stored rule set.
func (r Rules) Load() (*domain.Ruleset, bool, error) {
	b, ok, err := r.S.get("rules")
	if err != nil || !ok {
		return nil, false, err
	}
	var rs domain.Ruleset
	return &rs, true, json.Unmarshal(b, &rs)
}

// Save stores the rule set.
func (r Rules) Save(rs *domain.Ruleset) error { return r.S.put("rules", rs.JSON()) }

// Agreements is the AgreementRepo view.
type Agreements struct{ S *Store }

// Put stores an entry.
func (a Agreements) Put(e domain.Entry) error {
	b, _ := json.Marshal(e)
	_, err := a.S.db.ExecContext(bg, `INSERT INTO agreements(id, data) VALUES(?, ?) ON CONFLICT(id) DO UPDATE SET data = excluded.data`, e.AgreementID, b)
	return err
}

// Get returns an entry.
func (a Agreements) Get(id string) (domain.Entry, bool, error) {
	var b []byte
	err := a.S.db.QueryRowContext(bg, `SELECT data FROM agreements WHERE id = ?`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Entry{}, false, nil
	}
	if err != nil {
		return domain.Entry{}, false, err
	}
	var e domain.Entry
	return e, true, json.Unmarshal(b, &e)
}

// All returns every entry.
func (a Agreements) All() ([]domain.Entry, error) {
	rows, err := a.S.db.QueryContext(bg, `SELECT data FROM agreements ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var out []domain.Entry
	for rows.Next() {
		var b []byte
		if err := rows.Scan(&b); err != nil {
			return nil, err
		}
		var e domain.Entry
		if err := json.Unmarshal(b, &e); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}
