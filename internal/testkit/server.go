// Package testkit provides an in-process alexandria-server on the in-memory doubles, with the F6 namespace, principals and drafts.
// Server and client tests share it to publish real signed listings and packages without network (F8 §8.7).
package testkit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/server/adapters/compiler_poc"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Fixture identities of the F6 scenario.
const (
	Base    = "https://hub.example.com"
	NS      = "demo"
	NSIRI   = Base + "/v1/namespaces/demo"
	Context = Base + "/pub/_vhub/v1/context.jsonld"
	IdPIss  = "https://idp.example.com"
)

// Principals of F6: Ana is admin, Sergio is the steward of demo, Eve has a token but no permission.
var (
	Ana    = &domain.Principal{Iss: IdPIss, Sub: "ana", IsAdmin: true}
	Sergio = &domain.Principal{Iss: IdPIss, Sub: "sergio"}
	Eve    = &domain.Principal{Iss: IdPIss, Sub: "eve"}
)

// Draft builds a Part 5.1 draft of version v whose policy schema admits the given actions.
func Draft(v string, actions ...string) []byte {
	enum, _ := json.Marshal(actions)
	return []byte(`{"version":"` + v + `","profiles":["` + Base + `/pub/demo/profiles/odrl/` + v + `"],
"slots":[{"slot":"policy","obligation":"mandatory","applies_to":["policy_intake"]}],
"files":[{"path":"schemas/policy.schema.json","media_type":"application/schema+json","role":"json-schema","slot":"policy","applies_to":["policy_intake"],
"content":{"type":"object","properties":{"action":{"enum":` + string(enum) + `}},"required":["action"]}}]}`)
}

// DraftWith adds a lifecycle object to a draft.
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
	Hub      *app.Hub
	DB       *MemDB
	Clock    *FakeClock
	Signer   *KeySigner
	Resolver *Resolver
	Fetcher  *FakeFetcher
}

// NewServer builds a did:key hub whose clock starts at t0, with namespace demo and Sergio as its steward.
func NewServer(t *testing.T, t0 string) *Server {
	t.Helper()
	s := NewBareServer(t0)
	s.Setup(t)
	return s
}

// NewBareServer builds a did:key hub with compiler_poc, without namespaces and with its key checked.
func NewBareServer(t0 string) *Server {
	return NewBareServerWith(t0, compiler_poc.Compiler{Defaults: lifecycle.Defaults()})
}

// NewServerWith builds an F6 hub with namespace demo and the given compiler.
func NewServerWith(t *testing.T, t0 string, c app.ProfileCompiler) *Server {
	t.Helper()
	s := NewBareServerWith(t0, c)
	s.Setup(t)
	return s
}

// NewServerOn builds an F6 hub with compiler_poc on another store, keeping blobs in memory (contract tests of store adapters).
func NewServerOn(t *testing.T, t0 string, store app.Store) *Server {
	t.Helper()
	s := newServer(t0, compiler_poc.Compiler{Defaults: lifecycle.Defaults()}, store)
	s.Setup(t)
	return s
}

// NewBareServerWith builds a did:key hub with the given compiler.
func NewBareServerWith(t0 string, c app.ProfileCompiler) *Server { return newServer(t0, c, nil) }

// newServer builds a did:key hub on store, or on the memory store when nil.
func newServer(t0 string, c app.ProfileCompiler, store app.Store) *Server {
	db, clock, signer := NewMemDB(), NewClock(t0), NewKeySigner(1, "")
	if store == nil {
		store = db
	}
	s := &Server{DB: db, Clock: clock, Signer: signer, Resolver: NewResolver(), Fetcher: NewFakeFetcher()}
	s.Hub = app.New(app.Deps{
		Store: store, Blobs: MemBlobs{db}, Signer: signer, Resolver: s.Resolver, Fetcher: s.Fetcher,
		Compiler: c, Clock: clock, Base: Base,
		DID: signer.KeyDIDOf(), Context: Context, Defaults: lifecycle.Defaults(), VocabMaxSize: 1 << 16, AllowInsecureLocalhost: true,
	})
	s.Hub.Keys.CheckSigningKey()
	return s
}

// Setup creates namespace demo and assigns Sergio (F6.1 steps 1–2).
func (s *Server) Setup(t *testing.T) {
	t.Helper()
	if _, err := s.Hub.Namespaces.Create(Ana, app.NamespaceInput{Slug: NS}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Hub.Namespaces.AssignSteward(Ana, NS, app.StewardInput{Iss: Sergio.Iss, Sub: Sergio.Sub}); err != nil {
		t.Fatal(err)
	}
}

// Propose creates a draft of doc in demo and returns its candidate id.
func (s *Server) Propose(t *testing.T, doc []byte) string {
	t.Helper()
	d, err := s.Hub.Authoring.CreateDraft(Sergio, NS, doc)
	if err != nil {
		t.Fatal(err)
	}
	c, err := s.Hub.Authoring.Propose(Sergio, NS, d.ID)
	if err != nil {
		t.Fatal(err)
	}
	return c.ID
}

// Publish proposes and approves doc at instant at, returning the signed record.
func (s *Server) Publish(t *testing.T, at string, doc []byte, o app.ApproveOptions) []byte {
	t.Helper()
	s.Clock.Set(at)
	id := s.Propose(t, doc)
	rec, err := s.Hub.Release.Approve(Sergio, NS, id, o)
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

// PublishF6 publishes 1.0.0 on 2026-09-01 and 2.0.0 on 2026-10-01, as in F6.2.
func (s *Server) PublishF6(t *testing.T) {
	t.Helper()
	s.Publish(t, "2026-09-01T10:00:00Z", DraftV1, app.ApproveOptions{})
	s.Publish(t, "2026-10-01T10:00:00Z", DraftV2, app.ApproveOptions{})
}

// FakeFetcher is an UpstreamFetcher returning a settable response and counting calls (REQ-HUB-027).
type FakeFetcher struct {
	Body        []byte
	ContentType string
	Err         error
	Calls       int
}

// NewFakeFetcher returns a fetcher serving an empty JSON-LD context.
func NewFakeFetcher() *FakeFetcher {
	return &FakeFetcher{Body: []byte(`{"@context":{}}`), ContentType: "application/ld+json"}
}

// Fetch returns the configured response.
func (f *FakeFetcher) Fetch(string, int64) ([]byte, string, error) {
	f.Calls++
	return f.Body, f.ContentType, f.Err
}
