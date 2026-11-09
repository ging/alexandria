// Server domain: principals and the §3.6 role matrix, drafts, candidates, releases, revocations and events.
// Pure types and rules with no I/O; the authz matrix is checked cell by cell (REQ-HUB-002, REQ-HUB-003, REQ-HUB-017).
package domain

import (
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// Role is a /v1 role (§3.6).
type Role string

// Roles of §3.6.
const (
	Editor   Role = "editor"
	Releaser Role = "releaser"
	Member   Role = "member"
)

// Principal is an authenticated caller with its roles.
type Principal struct {
	Name  string `json:"principal"`
	Roles []Role `json:"roles"`
}

// Has reports whether the principal holds role r.
func (p Principal) Has(r Role) bool {
	for _, x := range p.Roles {
		if x == r {
			return true
		}
	}
	return false
}

// Action names one operation of §3.6.
type Action string

// Operations of the §3.6 table.
const (
	DraftCreate      Action = "POST /v1/drafts"
	DraftGet         Action = "GET /v1/drafts/{id}"
	DraftReplace     Action = "PUT /v1/drafts/{id}"
	DraftValidate    Action = "POST /v1/drafts/{id}:validate"
	DraftPropose     Action = "POST /v1/drafts/{id}:propose"
	CandidateGet     Action = "GET /v1/candidates/{id}"
	CandidateApprove Action = "POST /v1/candidates/{id}:approve"
	CandidateReject  Action = "POST /v1/candidates/{id}:reject"
	VersionsList     Action = "GET /v1/versions"
	VersionsGet      Action = "GET /v1/versions/{version}"
	VersionsRevoke   Action = "POST /v1/versions/{version}:revoke"
	EventsList       Action = "GET /v1/events"
)

// Matrix is the role column of §3.6.
var Matrix = map[Action][]Role{
	DraftCreate: {Editor}, DraftGet: {Editor}, DraftReplace: {Editor}, DraftValidate: {Editor}, DraftPropose: {Editor},
	CandidateGet: {Releaser}, CandidateApprove: {Releaser}, CandidateReject: {Releaser},
	VersionsList: {Member, Editor, Releaser}, VersionsGet: {Member, Editor, Releaser}, VersionsRevoke: {Releaser},
	EventsList: {Editor, Releaser},
}

// Allowed reports whether p may execute a.
func Allowed(p Principal, a Action) bool {
	for _, r := range Matrix[a] {
		if p.Has(r) {
			return true
		}
	}
	return false
}

// Draft is a mutable SpaceProfile draft under optimistic concurrency.
type Draft struct {
	ID   string `json:"id"`
	Rev  int    `json:"rev"`
	Doc  []byte `json:"-"`
	ETag string `json:"-"`
}

// CandidateState is open, approved or rejected.
type CandidateState string

// Candidate states of REQ-HUB-002.
const (
	Open     CandidateState = "open"
	Approved CandidateState = "approved"
	Rejected CandidateState = "rejected"
)

// Candidate is a frozen proposal: artefacts, diff, class and planned dates never change.
type Candidate struct {
	ID         string           `json:"id"`
	DraftID    string           `json:"draft_id"`
	State      CandidateState   `json:"state"`
	Comment    string           `json:"comment,omitempty"`
	Space      string           `json:"space"`
	Version    semver.Version   `json:"version"`
	Class      lifecycle.Class  `json:"class"`
	Manifest   ni.Digest        `json:"manifest"`
	Diff       []string         `json:"diff"`
	Planned    lifecycle.Record `json:"planned"`
	ProposedAt timex.Instant    `json:"proposed_at"`
	Params     lifecycle.Params `json:"lifecycle"`
	Package    Compiled         `json:"-"`
}

// Compiled is the output of the profile compiler: manifest, space-lock and files.
type Compiled struct {
	Space     string            `json:"space"`
	Version   semver.Version    `json:"version"`
	Class     lifecycle.Class   `json:"class"`
	Params    lifecycle.Params  `json:"lifecycle"`
	Manifest  []byte            `json:"manifest"`
	SpaceLock []byte            `json:"space_lock"`
	Files     map[string][]byte `json:"files"`
	Entries   []pkgv.File       `json:"entries"`
	Source    []byte            `json:"source,omitempty"`
}

// Release is an immutable published version.
type Release struct {
	Record      lifecycle.Record `json:"record"`
	Space       string           `json:"space"`
	Params      lifecycle.Params `json:"lifecycle"`
	Manifest    []byte           `json:"manifest"`
	ManifestSig []byte           `json:"manifest_sig"`
	Receipt     []byte           `json:"receipt"`
	SpaceLock   []byte           `json:"space_lock"`
	Entries     []pkgv.File      `json:"entries"`
	Source      []byte           `json:"source,omitempty"`
}

// Revocation is the signed addition of revoked_from to a release (REQ-HUB-007).
type Revocation struct {
	Version       semver.Version `json:"version"`
	RevokedFrom   timex.Instant  `json:"revoked_from"`
	Justification string         `json:"justification"`
	At            timex.Instant  `json:"at"`
	Principal     string         `json:"principal"`
	Envelope      []byte         `json:"envelope"`
}

// Event types of §3.6.
const (
	EvDraftCreated      = "draft.created"
	EvDraftUpdated      = "draft.updated"
	EvCandidateProposed = "candidate.proposed"
	EvCandidateApproved = "candidate.approved"
	EvCandidateRejected = "candidate.rejected"
	EvReleasePublished  = "release.published"
	EvReleaseAborted    = "release.aborted"
	EvVersionRevoked    = "version.revoked"
	EvListingIssued     = "listing.issued"
)

// Event is an append-only log entry with a strictly increasing id.
type Event struct {
	ID        int64         `json:"id"`
	TS        timex.Instant `json:"ts"`
	Type      string        `json:"type"`
	Principal string        `json:"principal"`
	Subject   string        `json:"subject"`
}

// SlugOf derives the /pub slug of a space IRI: its last non-empty path segment (agent decision).
func SlugOf(space string) string {
	s := space
	for len(s) > 0 && s[len(s)-1] == '/' {
		s = s[:len(s)-1]
	}
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '/' || s[i] == ':' {
			return s[i+1:]
		}
	}
	return s
}
