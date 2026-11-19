// Package store_postgres is the PostgreSQL adapter (pgx/v5/stdlib) of alexandria-client: continuity, rule set and agreement registry.
// State survives restarts; the rule set is stored whole with its revision (DP-2, REQ-CLI-005, REQ-CLI-031, REQ-SYS-006).
package store_postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	// Registers the pure-Go "pgx" database/sql driver used by Open.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
)

// bg is the context of every query.
var bg = context.Background()

const schema = `
CREATE TABLE IF NOT EXISTS kv(k TEXT PRIMARY KEY, v BYTEA NOT NULL);
CREATE TABLE IF NOT EXISTS agreements(id TEXT PRIMARY KEY, data BYTEA NOT NULL);
`

// Store implements ContinuityStore and backs Rules and Agreements views over PostgreSQL.
type Store struct{ db *sql.DB }

var (
	_ app.ContinuityStore = (*Store)(nil)
	_ app.RuleStore       = Rules{}
	_ app.AgreementRepo   = Agreements{}
)

// IsDSN reports whether target is a PostgreSQL connection URI.
func IsDSN(target string) bool {
	return strings.HasPrefix(target, "postgres://") || strings.HasPrefix(target, "postgresql://")
}

// Open connects to PostgreSQL, verifies reachability, and initializes the client tables.
func Open(dsn string) (*Store, error) {
	if !IsDSN(dsn) {
		return nil, errors.New("store_postgres: DSN must start with postgres:// or postgresql://")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)
	ctx, cancel := context.WithTimeout(bg, 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, err
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// Close closes the database connection pool.
func (s *Store) Close() error { return s.db.Close() }

// get reads a key; ok is false when absent.
func (s *Store) get(k string) ([]byte, bool, error) {
	var b []byte
	err := s.db.QueryRowContext(bg, `SELECT v FROM kv WHERE k = $1`, k).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return b, err == nil, err
}

// put writes or replaces a key.
func (s *Store) put(k string, b []byte) error {
	_, err := s.db.ExecContext(bg, `INSERT INTO kv(k, v) VALUES($1, $2) ON CONFLICT(k) DO UPDATE SET v = EXCLUDED.v`, k, b)
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

// Rules is the RuleStore view over Store.
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

// Agreements is the AgreementRepo view over Store.
type Agreements struct{ S *Store }

// Put stores or updates an agreement entry.
func (a Agreements) Put(e domain.Entry) error {
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	_, err = a.S.db.ExecContext(bg, `INSERT INTO agreements(id, data) VALUES($1, $2) ON CONFLICT(id) DO UPDATE SET data = EXCLUDED.data`, e.AgreementID, b)
	return err
}

// Get returns an agreement entry by id.
func (a Agreements) Get(id string) (domain.Entry, bool, error) {
	var b []byte
	err := a.S.db.QueryRowContext(bg, `SELECT data FROM agreements WHERE id = $1`, id).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Entry{}, false, nil
	}
	if err != nil {
		return domain.Entry{}, false, err
	}
	var e domain.Entry
	return e, true, json.Unmarshal(b, &e)
}

// All returns every agreement entry ordered by id.
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
