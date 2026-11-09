// In-process alexandria-server built on the in-memory doubles, plus F6 drafts and principals.
// Server and client tests share it to publish real signed listings and packages without network (F8 §8.7).
package testkit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/adapters/scitt_fake"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Fixture identities of the F6 scenario.
const (
	Space  = "https://example.com/spaces/demo"
	HubDID = did.DID("did:web:hub.example.com")
)

// Principals for each role.
var (
	Editor   = domain.Principal{Name: "steward", Roles: []domain.Role{domain.Editor}}
	Releaser = domain.Principal{Name: "releaser", Roles: []domain.Role{domain.Releaser}}
	Member   = domain.Principal{Name: "client", Roles: []domain.Role{domain.Member}}
)

// Draft builds a Part 5.1 draft of version v whose policy schema admits the given actions.
func Draft(v string, actions ...string) []byte {
	enum, _ := json.Marshal(actions)
	return []byte(`{"space":"` + Space + `","version":"` + v + `",
"slots":[{"slot":"odrl","obligation":"mandatory","profiles":["https://example.com/profiles/odrl/` + v + `"]}],
"files":[{"path":"schemas/policy.schema.json","media_type":"application/schema+json","role":"json-schema","slot":"odrl","applies_to":["policy_intake"],
"content":{"type":"object","properties":{"action":{"enum":` + string(enum) + `}},"required":["action"]}}]}`)
}

// DraftWith replaces the lifecycle object of a draft.
func DraftWith(doc []byte, lifecycleJSON string) []byte {
	return []byte(strings.Replace(string(doc), `"slots":`, `"lifecycle":`+lifecycleJSON+`,"slots":`, 1))
}

// F6 drafts: 1.0.0 admits odrl:use and ex:access, 2.0.0 admits odrl:use and odrl:read.
var (
	DraftV1 = Draft("1.0.0", "odrl:use", "ex:access")
	DraftV2 = Draft("2.0.0", "odrl:use", "odrl:read")
)

// Server is an in-process hub with its doubles exposed.
type Server struct {
	Hub    *app.Hub
	DB     *MemDB
	Clock  *FakeClock
	Signer *KeySigner
}

// NewServer builds a hub whose clock starts at t0.
func NewServer(t0 string) *Server {
	return NewServerWith(t0, compiler_poc.Compiler{Defaults: lifecycle.Defaults()})
}

// NewServerWith builds a hub whose clock starts at t0 and that compiles drafts with c.
func NewServerWith(t0 string, c app.ProfileCompiler) *Server {
	db := NewMemDB()
	clk := NewClock(t0)
	signer := NewKeySigner(HubDID)
	hub := app.New(app.Deps{
		Drafts: MemDrafts{db}, Candidates: MemCandidates{db}, Releases: MemReleases{db}, Listings: MemListings{db},
		Blobs: MemBlobs{db}, Log: FailingLog{scitt_fake.Log{Signer: signer, Clock: clk}, db}, Signer: signer,
		Compiler: c, Events: MemEvents{db}, Clock: clk, DID: HubDID,
		Defaults: lifecycle.Defaults(),
	})
	return &Server{Hub: hub, DB: db, Clock: clk, Signer: signer}
}

// Propose creates a draft and proposes it, returning the candidate id.
func (s *Server) Propose(t *testing.T, doc []byte) string {
	t.Helper()
	d, err := s.Hub.Authoring.CreateDraft(Editor, doc)
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}
	c, err := s.Hub.Authoring.Propose(Editor, d.ID)
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	return c.ID
}

// Publish proposes and approves a draft at instant at, returning the signed record.
func (s *Server) Publish(t *testing.T, at string, doc []byte, o app.ApproveOptions) []byte {
	t.Helper()
	s.Clock.Set(at)
	rec, err := s.Hub.Release.Approve(Releaser, s.Propose(t, doc), o)
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	return rec
}

// PublishF6 publishes 1.0.0 and 2.0.0 at the F6 instants.
func (s *Server) PublishF6(t *testing.T) {
	t.Helper()
	s.Publish(t, "2026-09-01T10:00:00Z", DraftV1, app.ApproveOptions{})
	s.Publish(t, "2026-10-01T10:00:00Z", DraftV2, app.ApproveOptions{})
}
