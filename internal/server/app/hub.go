// Package app composes the server use cases around shared ports and maps failures to coded errors.
// Each use case authorizes with the §3.5 table and records an event for every effect (REQ-HUB-017, REQ-HUB-020, REQ-HUB-023).
package app

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/core/vc"
	"github.com/ging/alexandria/internal/server/domain"
)

// Deps groups the outbound ports and settings shared by the use cases.
type Deps struct {
	Store                  Store
	Blobs                  BlobStore
	Signer                 KeySigner
	Resolver               DIDResolver
	Fetcher                UpstreamFetcher
	Compiler               ProfileCompiler
	Clock                  timex.Clock
	Logger                 *slog.Logger
	Base                   string
	DID                    did.DID
	Context                string
	Defaults               lifecycle.Params
	VocabMaxSize           int64
	AllowInsecureLocalhost bool
}

// Hub bundles the use cases of alexandria-server.
type Hub struct {
	Namespaces   *Namespaces
	Vocabularies *Vocabularies
	VocabReader  *VocabReader
	Authoring    *Authoring
	Release      *Release
	Revocation   *Revocation
	Listing      *Listing
	Query        *Query
	Publication  *Publication
	Keys         *KeyGate
}

// New wires the use cases; read-only vocabulary access is built without the upstream fetcher (REQ-HUB-027).
func New(d Deps) *Hub {
	if d.VocabMaxSize == 0 {
		d.VocabMaxSize = 10 << 20
	}
	keys := &KeyGate{d: d}
	l := &Listing{d: d, keys: keys, mu: &sync.Mutex{}}
	readOnly := d
	readOnly.Fetcher = nil
	reader := &VocabReader{d: readOnly}
	return &Hub{
		Namespaces:   &Namespaces{d: d},
		Vocabularies: &Vocabularies{d: d, reader: reader},
		VocabReader:  reader,
		Authoring:    &Authoring{d: d, reader: reader},
		Release:      &Release{d: d, keys: keys, listing: l},
		Revocation:   &Revocation{d: d, keys: keys, listing: l},
		Listing:      l,
		Query:        &Query{d: d},
		Publication:  &Publication{d: readOnly, reader: reader},
		Keys:         keys,
	}
}

// nsIRI returns the IRI of a namespace slug.
func (d Deps) nsIRI(slug string) string { return d.Base + "/v1/namespaces/" + slug }

// log returns the configured logger or a no-op fallback.
func (d Deps) log() *slog.Logger { return logx.Default(d.Logger) }

// namespace returns an existing namespace or API_NOT_FOUND.
func (d Deps) namespace(slug string) (domain.Namespace, error) {
	n, err := d.Store.GetNamespace(slug)
	return n, notFound(err, "namespace "+slug)
}

// authorize applies 401, then the namespace existence (404), then the §3.5 table (403).
func (d Deps) authorize(op domain.Op, p *domain.Principal, ns string) error {
	if op != domain.OpPublic && p == nil {
		return codes.New(codes.APIUnauthenticated, "this operation requires a bearer token")
	}
	steward := false
	if ns != "" {
		if _, err := d.namespace(ns); err != nil {
			return err
		}
		if p != nil {
			ok, err := d.Store.IsSteward(ns, p.Iss, p.Sub)
			if err != nil {
				return err
			}
			steward = ok
		}
	}
	if domain.Decide(op, p, steward) == domain.Forbidden {
		d.log().Warn("authorization denied", slog.String("principal", p.String()), slog.String("namespace", ns), logx.Code(codes.APIForbiddenRole))
		return codes.New(codes.APIForbiddenRole, "the principal has no permission for this operation")
	}
	return nil
}

// emit appends an event; the principal is "vhub" for system events.
func (d Deps) emit(p *domain.Principal, typ, ns, subject string) error {
	who := "vhub"
	if p != nil {
		who = p.String()
	}
	_, err := d.Store.Append(domain.Event{TS: d.Clock.Now(), Principal: who, Type: typ, Namespace: ns, Subject: subject})
	if err != nil {
		d.log().Error("failed to append audit event", slog.String("event_type", typ), slog.String("namespace", ns), logx.Err(err))
	}
	return err
}

// unsigned serializes an unsigned credential of the hub.
func (d Deps) unsigned(t string, from timex.Instant, until *timex.Instant, subject any) []byte {
	b, _ := json.Marshal(vc.New(t, d.Context, string(d.DID), from, until, subject))
	return b
}

// notFound maps a repository miss to API_NOT_FOUND and passes other errors through.
func notFound(err error, what string) error {
	if errors.Is(err, ErrNotFound) {
		return codes.New(codes.APINotFound, "%s not found", what)
	}
	return err
}

// records returns the release records of a namespace with revocations applied.
func (d Deps) records(ns string) ([]lifecycle.Record, []domain.Release, error) {
	rels, err := d.Store.Releases(ns)
	if err != nil {
		return nil, nil, err
	}
	out := make([]lifecycle.Record, 0, len(rels))
	for _, r := range rels {
		out = append(out, r.Record)
	}
	return out, rels, nil
}

// headParams returns the lifecycle of the greatest non-revoked release, or the defaults.
func (d Deps) headParams(ns string) (lifecycle.Params, error) {
	recs, rels, err := d.records(ns)
	if err != nil {
		return lifecycle.Params{}, err
	}
	if g, ok := lifecycle.Greatest(recs, true); ok {
		for _, r := range rels {
			if semver.Compare(r.Record.Version, g.Version) == 0 {
				return r.Params, nil
			}
		}
	}
	return d.Defaults, nil
}

// newID returns a random identifier.
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
