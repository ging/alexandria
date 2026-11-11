// Package app declares the outbound ports of alexandria-client and the request and response types that cross the proxy boundary (F8 §8.5).
// Edge and Explainer never receive HubAPI or DIDResolver, which is how the hot path cannot call the hub (REQ-SYS-002, REQ-CLI-005, REQ-CLI-017, REQ-CLI-031).
package app

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// ErrNamespaceNotFound is returned by HubAPI.Listing when the hub answers 404 (S1).
var ErrNamespaceNotFound = errors.New("namespace not found")

// ListingResponse is the raw listing answer with its Date header.
type ListingResponse struct {
	Body    []byte
	Date    timex.Instant
	HasDate bool
}

// HubAPI is the unauthenticated client of the hub, used only by Sync.
type HubAPI interface {
	Listing() (ListingResponse, error)
	Package(d ni.Digest) (pkgv.Files, error)
}

// DIDResolver resolves the hub DID (did:web over the network, did:key locally).
type DIDResolver interface {
	Resolve(d did.DID) (did.Document, error)
}

// Evaluator evaluates a message against one package for one gate, without network.
type Evaluator interface {
	Evaluate(m domain.Message, g domain.Gate, p *pkgv.Package) domain.EvalResult
}

// Continuity is the state that must survive restarts (REQ-CLI-005).
type Continuity struct {
	Accepted   *lifecycle.Accepted `json:"accepted"`
	DID        *did.Document       `json:"did_document"`
	ResolvedAt timex.Instant       `json:"resolved_at"`
}

// ContinuityStore persists Continuity.
type ContinuityStore interface {
	Load() (Continuity, error)
	Save(c Continuity) error
}

// PackageStore keeps verified package files by manifest digest; only GC deletes.
type PackageStore interface {
	Get(d ni.Digest) (pkgv.Files, bool, error)
	Put(d ni.Digest, f pkgv.Files) error
	Delete(d ni.Digest) error
	List() ([]ni.Digest, error)
}

// RuleStore persists the rule set (REQ-CLI-031); Load reports false when the store is empty.
type RuleStore interface {
	Load() (*domain.Ruleset, bool, error)
	Save(rs *domain.Ruleset) error
}

// AgreementReader reads the registry; Explain only gets this view.
type AgreementReader interface {
	Get(id string) (domain.Entry, bool, error)
}

// AgreementRepo stores the agreement registry.
type AgreementRepo interface {
	AgreementReader
	Put(e domain.Entry) error
	All() ([]domain.Entry, error)
}

// EventSink receives events.
type EventSink interface {
	Emit(e any)
}

// Metrics receives classification counts.
type Metrics interface {
	Classified(rule, gate, version, result string)
}

// Request is a message received on a traffic listener; Target is set for dsp-egress.
type Request struct {
	Listener domain.Listener
	Method   string
	Path     string
	RawQuery string
	Host     string
	Header   http.Header
	Body     []byte
	Target   *url.URL
}

// Response is what the client returns to the sender.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Upstream forwards a request unchanged to the connector or the egress destination.
type Upstream interface {
	Forward(r *Request) (*Response, error)
}
