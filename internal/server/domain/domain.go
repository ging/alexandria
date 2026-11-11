// Package domain holds the server domain: namespaces, stewards, principals, vocabularies, drafts, candidates, releases and events.
// Pure types with no I/O; the authorization table of §3.5 lives in authz.go (REQ-SYS-010, REQ-HUB-002, REQ-HUB-025).
package domain

import (
	"regexp"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/semver"
	"github.com/ging/alexandria/internal/core/timex"
)

// ReservedSlug is the slug used by /pub/_vhub (§3.2).
const ReservedSlug = "_vhub"

// slugRe and vocabNameRe are the patterns of §3.2 and §3.6.
var (
	slugRe      = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`)
	vocabNameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{0,127}$`)
)

// ValidSlug reports whether s matches the namespace slug pattern.
func ValidSlug(s string) bool { return slugRe.MatchString(s) }

// ValidVocabName reports whether s matches the vocabulary name pattern.
func ValidVocabName(s string) bool { return vocabNameRe.MatchString(s) }

// Namespace is the unit of logical separation of the hub (§3.2).
type Namespace struct {
	Slug        string        `json:"slug"`
	ID          string        `json:"id"`
	Title       string        `json:"title,omitempty"`
	Description string        `json:"description,omitempty"`
	CreatedAt   timex.Instant `json:"created_at"`
}

// Steward is a principal assigned to a namespace (§3.5).
type Steward struct {
	ID         string        `json:"id"`
	Iss        string        `json:"iss"`
	Sub        string        `json:"sub"`
	Label      string        `json:"label,omitempty"`
	AssignedAt timex.Instant `json:"assigned_at"`
	AssignedBy string        `json:"assigned_by"`
}

// Principal is the authenticated (iss, sub) pair; IsAdmin comes from the configured admin claim.
type Principal struct {
	Iss     string
	Sub     string
	IsAdmin bool
}

// String renders the principal for events.
func (p Principal) String() string { return p.Iss + "#" + p.Sub }

// Vocabulary kinds of §3.6.
const (
	Hosted = "hosted"
	Proxy  = "proxy"
)

// Revision is one immutable revision of a vocabulary.
type Revision struct {
	N         int           `json:"n"`
	Digest    ni.Digest     `json:"digest"`
	Size      int64         `json:"size"`
	CreatedAt timex.Instant `json:"created_at"`
	CreatedBy string        `json:"created_by"`
}

// Vocabulary is an informative resource of a namespace, hosted or proxied.
type Vocabulary struct {
	Name        string     `json:"name"`
	Kind        string     `json:"kind"`
	MediaType   string     `json:"media_type"`
	Upstream    string     `json:"upstream,omitempty"`
	Title       string     `json:"title,omitempty"`
	Description string     `json:"description,omitempty"`
	Revisions   []Revision `json:"revisions"`
}

// Latest returns the last revision, if any.
func (v Vocabulary) Latest() (Revision, bool) {
	if len(v.Revisions) == 0 {
		return Revision{}, false
	}
	return v.Revisions[len(v.Revisions)-1], true
}

// Draft is a mutable SpaceProfile draft under optimistic concurrency.
type Draft struct {
	ID        string `json:"id"`
	Namespace string `json:"namespace"`
	Rev       int    `json:"rev"`
	Doc       []byte `json:"-"`
	ETag      string `json:"-"`
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
	Namespace  string           `json:"namespace"`
	DraftID    string           `json:"draft_id"`
	State      CandidateState   `json:"state"`
	Comment    string           `json:"comment,omitempty"`
	Version    semver.Version   `json:"version"`
	Class      lifecycle.Class  `json:"class"`
	Diff       []string         `json:"diff"`
	Planned    lifecycle.Record `json:"planned"`
	ProposedAt timex.Instant    `json:"proposed_at"`
	Params     lifecycle.Params `json:"lifecycle"`
	Package    Compiled         `json:"-"`
}

// Compiled is the output of the profile compiler: the files and their manifest entries, unsigned.
type Compiled struct {
	Version  semver.Version    `json:"version"`
	Class    lifecycle.Class   `json:"class"`
	Params   lifecycle.Params  `json:"lifecycle"`
	Profiles []string          `json:"profiles,omitempty"`
	Entries  []pkgv.File       `json:"entries"`
	Files    map[string][]byte `json:"files"`
	Source   []byte            `json:"source,omitempty"`
}

// Release is an immutable published version with its signed manifest and its signed record.
type Release struct {
	Namespace string           `json:"namespace"`
	Record    lifecycle.Record `json:"record"`
	Params    lifecycle.Params `json:"lifecycle"`
	Manifest  []byte           `json:"manifest"`
	SignedRec []byte           `json:"signed_record"`
	Entries   []pkgv.File      `json:"entries"`
	Source    []byte           `json:"source,omitempty"`
}

// Revocation is the addition of revoked_from to a release, with its re-signed record (REQ-HUB-007).
type Revocation struct {
	Namespace     string         `json:"namespace"`
	Version       semver.Version `json:"version"`
	RevokedFrom   timex.Instant  `json:"revoked_from"`
	Justification string         `json:"justification"`
	At            timex.Instant  `json:"at"`
	Principal     string         `json:"principal"`
	SignedRec     []byte         `json:"signed_record"`
}

// Event types of §3.9.
const (
	EvNamespaceCreated     = "namespace.created"
	EvStewardAssigned      = "steward.assigned"
	EvStewardRemoved       = "steward.removed"
	EvSignerKeyUnpublished = "signer.key_unpublished"
	EvVocabularyCreated    = "vocabulary.created"
	EvVocabularyRevised    = "vocabulary.revised"
	EvDraftCreated         = "draft.created"
	EvDraftUpdated         = "draft.updated"
	EvCandidateProposed    = "candidate.proposed"
	EvCandidateApproved    = "candidate.approved"
	EvCandidateRejected    = "candidate.rejected"
	EvReleasePublished     = "release.published"
	EvReleaseAborted       = "release.aborted"
	EvVersionRevoked       = "version.revoked"
	EvListingIssued        = "listing.issued"
)

// Event is an append-only log entry with a strictly increasing id; Namespace is empty for global events.
type Event struct {
	ID        int64         `json:"id"`
	TS        timex.Instant `json:"ts"`
	Principal string        `json:"principal"`
	Type      string        `json:"type"`
	Namespace string        `json:"namespace,omitempty"`
	Subject   string        `json:"subject"`
}
