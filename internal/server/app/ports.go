// Outbound ports of vhub-server, declared by the application that needs them (F8 §8.4).
// sqlite, blob_fs, signer_file, scitt_fake and compiler_poc implement them; testkit provides in-memory doubles (REQ-HUB-004, REQ-HUB-013).
package app

import (
	"errors"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/sign"
	"github.com/ging/alexandria/internal/server/domain"
)

// Sentinel errors returned by repositories.
var (
	ErrNotFound           = errors.New("not found")
	ErrPreconditionFailed = errors.New("precondition failed")
	ErrInvalidState       = errors.New("invalid state")
	ErrSequenceTaken      = errors.New("sequence already issued")
)

// DraftRepo stores mutable drafts with optimistic concurrency.
type DraftRepo interface {
	Create(doc []byte) (domain.Draft, error)
	Get(id string) (domain.Draft, error)
	Replace(id, ifMatch string, doc []byte) (domain.Draft, error)
}

// CandidateRepo stores frozen candidates; decisions are appended, never updated.
type CandidateRepo interface {
	Insert(c domain.Candidate) error
	Get(id string) (domain.Candidate, error)
	Reject(id, comment string) error
}

// ReleaseStore only appends releases and revocations (REQ-HUB-013).
type ReleaseStore interface {
	AppendRelease(r domain.Release, fromCandidate string) error
	AppendRevocation(rv domain.Revocation) error
	Records() ([]lifecycle.Record, error)
	Release(v semver.Version) (domain.Release, error)
	ByManifest(d ni.Digest) (domain.Release, error)
	Releases() ([]domain.Release, error)
	HasBlob(hex string) (bool, error)
}

// StoredListing is an issued listing envelope with its sequence and issue instant.
type StoredListing struct {
	Sequence uint64
	Raw      []byte
	Listing  lifecycle.Listing
}

// ListingStore keeps every issued listing; sequence uniqueness is enforced by the store.
type ListingStore interface {
	Put(l StoredListing) error
	Latest() (StoredListing, bool, error)
}

// BlobStore is a content-addressed byte store.
type BlobStore interface {
	Put(hex string, b []byte) error
	Get(hex string) ([]byte, error)
}

// TransparencyLog registers manifest digests and returns a receipt (shape: Q2, DP-8).
type TransparencyLog interface {
	Register(d ni.Digest) ([]byte, error)
}

// Signer signs only with keys present in the hub DID document (REQ-HUB-015).
type Signer interface {
	PublishedKeys() []did.PublicKey
	Sign(msg []byte) (sign.Signature, error)
}

// CompileContext is what N4 and N2 need to know about published versions.
type CompileContext struct {
	Latest   *semver.Version
	Space    string
	Previous []byte
}

// ProfileCompiler validates drafts (N1–N5) and compiles them into a package (DP-6).
type ProfileCompiler interface {
	Validate(doc []byte, level int, ctx CompileContext) []*codes.Error
	Compile(doc []byte, ctx CompileContext) (domain.Compiled, error)
}

// EventLog appends events with strictly increasing ids.
type EventLog interface {
	Append(e domain.Event) (int64, error)
	After(after int64, limit int) ([]domain.Event, error)
}
