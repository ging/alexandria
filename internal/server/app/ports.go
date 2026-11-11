// Package app declares the outbound ports of alexandria-server, owned by the application that needs them (F8 §8.4).
// Every namespaced method takes the namespace slug, so no operation can reach another namespace (REQ-SYS-010, REQ-HUB-013, REQ-HUB-024).
package app

import (
	"crypto/ed25519"
	"errors"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/domain"
)

// Sentinel errors returned by repositories.
var (
	ErrNotFound           = errors.New("not found")
	ErrExists             = errors.New("already exists")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrInvalidState       = errors.New("invalid state")
	ErrSequenceTaken      = errors.New("sequence already issued")
)

// NamespaceRepo stores namespaces; they are never renamed nor deleted.
type NamespaceRepo interface {
	CreateNamespace(n domain.Namespace) error
	GetNamespace(slug string) (domain.Namespace, error)
	ListNamespaces() ([]domain.Namespace, error)
}

// StewardRepo stores the stewards of each namespace; IsSteward is read on every request (REQ-HUB-023).
type StewardRepo interface {
	AddSteward(ns string, s domain.Steward) error
	RemoveSteward(ns, id string) error
	IsSteward(ns, iss, sub string) (bool, error)
	ListStewards(ns string) ([]domain.Steward, error)
}

// VocabRepo stores vocabularies; revisions are only appended (REQ-HUB-013).
type VocabRepo interface {
	CreateVocabulary(ns string, v domain.Vocabulary) error
	GetVocabulary(ns, name string) (domain.Vocabulary, error)
	AppendRevision(ns, name string, r domain.Revision) error
	ListVocabularies(ns string) ([]domain.Vocabulary, error)
}

// DraftRepo stores mutable drafts with optimistic concurrency.
type DraftRepo interface {
	CreateDraft(ns string, doc []byte) (domain.Draft, error)
	GetDraft(ns, id string) (domain.Draft, error)
	ReplaceDraft(ns, id, ifMatch string, doc []byte) (domain.Draft, error)
}

// CandidateRepo stores frozen candidates; decisions are appended, never updated.
type CandidateRepo interface {
	InsertCandidate(c domain.Candidate) error
	GetCandidate(ns, id string) (domain.Candidate, error)
	RejectCandidate(ns, id, comment string) error
}

// ReleaseStore only appends releases and revocations (REQ-HUB-013).
type ReleaseStore interface {
	AppendRelease(r domain.Release, fromCandidate string) error
	AppendRevocation(rv domain.Revocation) error
	Releases(ns string) ([]domain.Release, error)
	ByManifest(d ni.Digest) (domain.Release, error)
}

// StoredListing is an issued listing credential with its subject and validity.
type StoredListing struct {
	Sequence   uint64
	Raw        []byte
	IssuedAt   timex.Instant
	ValidUntil timex.Instant
	Subject    lifecycle.ListingSubject
}

// ListingStore keeps every issued listing; (namespace, sequence) uniqueness is enforced by the store (REQ-HUB-008).
type ListingStore interface {
	PutListing(ns string, l StoredListing) error
	LatestListing(ns string) (StoredListing, bool, error)
}

// BlobStore is a content-addressed byte store.
type BlobStore interface {
	Put(hex string, b []byte) error
	Get(hex string) ([]byte, error)
}

// KeySigner holds the active Ed25519 key and its verification method (PRE-06).
type KeySigner interface {
	VerificationMethod() string
	PublicKey() ed25519.PublicKey
	Sign(unsecured []byte, created timex.Instant) ([]byte, error)
}

// DIDResolver resolves the hub DID document to check that the active key is published (REQ-HUB-015).
type DIDResolver interface {
	Resolve(d did.DID) (did.Document, error)
}

// UpstreamFetcher downloads a proxied vocabulary; only Vocabularies.Create and Refresh may call it (REQ-HUB-027).
type UpstreamFetcher interface {
	Fetch(url string, limit int64) (body []byte, contentType string, err error)
}

// CompileContext is what the compiler needs: the namespace IRI, the latest published version and its source, and a vocabulary reader.
type CompileContext struct {
	Namespace string
	Latest    *semver.Version
	Previous  []byte
	Vocab     func(iri string) ([]byte, error)
}

// ProfileCompiler validates drafts (N1–N5) and compiles them into a package (DP-6).
type ProfileCompiler interface {
	Validate(doc []byte, level int, ctx CompileContext) []*codes.Error
	Compile(doc []byte, ctx CompileContext) (domain.Compiled, error)
}

// EventLog appends events with strictly increasing ids; ns "" selects global events.
type EventLog interface {
	Append(e domain.Event) (int64, error)
	After(ns string, after int64, limit int) ([]domain.Event, error)
}

// Store groups the persistence ports implemented by one database adapter.
type Store interface {
	NamespaceRepo
	StewardRepo
	VocabRepo
	DraftRepo
	CandidateRepo
	ReleaseStore
	ListingStore
	EventLog
}
