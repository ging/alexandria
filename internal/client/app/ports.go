// Outbound ports of vhub-client and the request/response types that cross the proxy boundary (F8 §8.5).
// Edge never receives HubAPI or DIDResolver, which is how the hot path cannot call the hub (REQ-SYS-002, REQ-CLI-005, REQ-CLI-017).
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

// Hub access errors distinguished by S1.
var (
	ErrUnreachable  = errors.New("hub unreachable")
	ErrUnauthorized = errors.New("hub refused credentials")
)

// ListingResponse is the raw GET /v1/versions answer with its Date header.
type ListingResponse struct {
	Body    []byte
	Date    timex.Instant
	HasDate bool
}

// HubAPI is the authenticated client of the hub, used only by Sync.
type HubAPI interface {
	Listing() (ListingResponse, error)
	Package(d ni.Digest) (pkgv.Files, error)
}

// DIDResolver resolves the hub did:web.
type DIDResolver interface {
	Resolve(d did.DID) (did.Document, error)
}

// Evaluator evaluates a selected body against one package for one gate, without network.
type Evaluator interface {
	Evaluate(body []byte, g domain.Gate, p *pkgv.Package) domain.EvalResult
}

// ReceiptVerifier checks manifest.receipt for a digest with the hub keys (shape: Q2, DP-8).
type ReceiptVerifier interface {
	Verify(d ni.Digest, receipt []byte, keys []did.PublicKey) error
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

// PackageStore keeps verified package files by manifest digest (REQ-CLI-017).
type PackageStore interface {
	Get(d ni.Digest) (pkgv.Files, bool, error)
	Put(d ni.Digest, f pkgv.Files) error
	Retain(keep []ni.Digest) error
}

// AgreementRepo stores the agreement registry.
type AgreementRepo interface {
	Put(e domain.Entry) error
	Get(id string) (domain.Entry, bool, error)
	All() ([]domain.Entry, error)
}

// EventSink receives events.
type EventSink interface {
	Emit(e any)
}

// Metrics receives classification counts.
type Metrics interface {
	Classified(gate, version, result string)
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
