// Package testkit provides an in-process alexandria-client wired to the in-process hub, with in-memory stores, a fake upstream and an event recorder.
// It also forges listings and packages re-signed with the hub key to build the S1–S13 negative corpus (F8 §8.7, REQ-CLI-004..007).
package testkit

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/ging/alexandria/internal/client/adapters/evaluator_poc"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/lifecycle"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/vc"
)

// FakeHub serves the in-process hub to the client; hooks forge or break the answers.
type FakeHub struct {
	S            *Server
	Down         bool
	ListingHook  func([]byte) []byte
	ManifestHook func(ni.Digest, []byte) []byte
	FilesHook    func(map[string][]byte)
	PackageHook  func(ni.Digest) (pkgv.Files, bool)
	Calls        int
}

// Listing returns the current listing of demo as the hub serves it.
func (h *FakeHub) Listing() (app.ListingResponse, error) {
	h.Calls++
	if h.Down {
		return app.ListingResponse{}, errors.New("connection refused")
	}
	b, err := h.S.Hub.Listing.Latest(NS)
	if ce, ok := codes.As(err); ok && ce.Code == codes.APINotFound {
		return app.ListingResponse{}, app.ErrNamespaceNotFound
	}
	if err != nil {
		return app.ListingResponse{}, err
	}
	if h.ListingHook != nil {
		b = h.ListingHook(b)
	}
	return app.ListingResponse{Body: b, Date: h.S.Clock.Now(), HasDate: true}, nil
}

// Package returns the manifest and blobs of a package.
func (h *FakeHub) Package(d ni.Digest) (pkgv.Files, error) {
	if h.Down {
		return pkgv.Files{}, errors.New("connection refused")
	}
	if h.PackageHook != nil {
		if f, ok := h.PackageHook(d); ok {
			return f, nil
		}
	}
	m, err := h.S.Hub.Publication.Manifest(d.Segment())
	if err != nil {
		return pkgv.Files{}, err
	}
	c, err := pkgv.Parse(m)
	if err != nil {
		return pkgv.Files{}, err
	}
	files := map[string][]byte{}
	for _, f := range c.Subject.Files {
		b, err := h.S.Hub.Publication.Blob(f.Digest.Hex())
		if err != nil {
			return pkgv.Files{}, err
		}
		files[f.Path] = b
	}
	if h.ManifestHook != nil {
		m = h.ManifestHook(d, m)
	}
	if h.FilesHook != nil {
		h.FilesHook(files)
	}
	return pkgv.Files{Manifest: m, Content: files}, nil
}

// Resign re-signs a secured document after mutating its decoded form, with the hub key.
func (s *Server) Resign(t *testing.T, raw []byte, mutate func(map[string]any)) []byte {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	delete(doc, "proof")
	mutate(doc)
	b, _ := json.Marshal(doc)
	var head struct {
		ValidFrom string `json:"validFrom"`
	}
	_ = json.Unmarshal(b, &head)
	created, _ := timexParse(head.ValidFrom)
	out, err := s.Signer.Sign(b, created)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// Subject returns the credentialSubject map of a decoded credential.
func Subject(doc map[string]any) map[string]any {
	m, _ := doc["credentialSubject"].(map[string]any)
	return m
}

// Recorder is an EventSink keeping every event.
type Recorder struct {
	mu     sync.Mutex
	Events []any
}

// Emit records an event.
func (r *Recorder) Emit(e any) { r.mu.Lock(); r.Events = append(r.Events, e); r.mu.Unlock() }

// Codes returns the codes of every recorded event.
func (r *Recorder) Codes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.Events {
		switch v := e.(type) {
		case app.Notice:
			out = append(out, v.Code)
		case app.Verdict:
			out = append(out, v.Codes...)
		}
	}
	return out
}

// Has reports whether some event carries code.
func (r *Recorder) Has(code string) bool { return slices.Contains(r.Codes(), code) }

// Count returns how many events carry code.
func (r *Recorder) Count(code string) int {
	n := 0
	for _, c := range r.Codes() {
		if c == code {
			n++
		}
	}
	return n
}

// Notices returns the notices of a type.
func (r *Recorder) Notices(typ string) []app.Notice {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Notice
	for _, e := range r.Events {
		if n, ok := e.(app.Notice); ok && n.Type == typ {
			out = append(out, n)
		}
	}
	return out
}

// Verdicts returns the recorded vhub.verdict events.
func (r *Recorder) Verdicts() []app.Verdict {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Verdict
	for _, e := range r.Events {
		if v, ok := e.(app.Verdict); ok {
			out = append(out, v)
		}
	}
	return out
}

// Reset forgets the recorded events.
func (r *Recorder) Reset() { r.mu.Lock(); r.Events = nil; r.mu.Unlock() }

// FakeUpstream records forwarded requests and answers with a settable response.
type FakeUpstream struct {
	mu       sync.Mutex
	Status   int
	Body     []byte
	Header   http.Header
	Received []*app.Request
}

// Forward records the request and answers.
func (u *FakeUpstream) Forward(r *app.Request) (*app.Response, error) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.Received = append(u.Received, r)
	st := u.Status
	if st == 0 {
		st = http.StatusOK
	}
	h := http.Header{"Content-Type": {"application/json"}}
	for k, v := range u.Header {
		h[k] = v
	}
	body := u.Body
	if body == nil {
		body = []byte(`{}`)
	}
	return &app.Response{Status: st, Header: h, Body: body}, nil
}

// Count returns the number of forwarded requests.
func (u *FakeUpstream) Count() int { u.mu.Lock(); defer u.mu.Unlock(); return len(u.Received) }

// MemContinuity is an in-memory ContinuityStore.
type MemContinuity struct {
	mu sync.Mutex
	b  []byte
}

// Load returns the saved continuity.
func (m *MemContinuity) Load() (app.Continuity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var c app.Continuity
	if m.b == nil {
		return c, nil
	}
	return c, json.Unmarshal(m.b, &c)
}

// Save stores the continuity as JSON, like a file store.
func (m *MemContinuity) Save(c app.Continuity) error {
	b, err := json.Marshal(c)
	m.mu.Lock()
	m.b = b
	m.mu.Unlock()
	return err
}

// MemPackages is an in-memory PackageStore.
type MemPackages struct {
	mu sync.Mutex
	m  map[ni.Digest]pkgv.Files
}

// Get returns a stored package.
func (p *MemPackages) Get(d ni.Digest) (pkgv.Files, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.m[d]
	return f, ok, nil
}

// Put stores a package.
func (p *MemPackages) Put(d ni.Digest, f pkgv.Files) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[ni.Digest]pkgv.Files{}
	}
	p.m[d] = f
	return nil
}

// Delete removes a package.
func (p *MemPackages) Delete(d ni.Digest) error {
	p.mu.Lock()
	delete(p.m, d)
	p.mu.Unlock()
	return nil
}

// List returns the stored digests.
func (p *MemPackages) List() ([]ni.Digest, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []ni.Digest
	for d := range p.m {
		out = append(out, d)
	}
	return out, nil
}

// MemAgreements is an in-memory AgreementRepo.
type MemAgreements struct {
	mu sync.Mutex
	m  map[string]domain.Entry
}

// Put stores an entry.
func (a *MemAgreements) Put(e domain.Entry) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.m == nil {
		a.m = map[string]domain.Entry{}
	}
	a.m[e.AgreementID] = e
	return nil
}

// Get returns an entry.
func (a *MemAgreements) Get(id string) (domain.Entry, bool, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	e, ok := a.m[id]
	return e, ok, nil
}

// All returns every entry.
func (a *MemAgreements) All() ([]domain.Entry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	var out []domain.Entry
	for _, e := range a.m {
		out = append(out, e)
	}
	return out, nil
}

// MemRules is an in-memory RuleStore.
type MemRules struct{ rs *domain.Ruleset }

// Load returns the stored rule set.
func (m *MemRules) Load() (*domain.Ruleset, bool, error) { return m.rs, m.rs != nil, nil }

// Save stores the rule set.
func (m *MemRules) Save(rs *domain.Ruleset) error { m.rs = rs; return nil }

// noMetrics discards metrics.
type noMetrics struct{}

// Classified does nothing.
func (noMetrics) Classified(string, string, string, string) {}

// Client is an in-process client with its doubles exposed.
type Client struct {
	Server     *Server
	Hub        *FakeHub
	Events     *Recorder
	Upstream   *FakeUpstream
	Continuity *MemContinuity
	Packages   *MemPackages
	Repo       *MemAgreements
	RuleStore  *MemRules
	Cell       *app.Cell
	RulesCell  *app.RulesCell
	Trust      *app.HubTrust
	Sync       *app.Sync
	Edge       *app.Edge
	Rules      *app.Rules
	Agreements *app.Agreements
	Audit      *app.Audit
	Status     *app.Status
	GC         *app.GC
}

// NewClient builds a client of demo trusting the hub DID, with the Annex C seed (contract-request in enforce).
func NewClient(t *testing.T, s *Server) *Client {
	t.Helper()
	return NewClientWith(t, s, SeedPath(t), "forward", "forward")
}

// NewClientWith builds a client with a seed file and the default and unknown-agreement actions.
func NewClientWith(t *testing.T, s *Server, seed, defaultAction, unknown string) *Client {
	t.Helper()
	c := &Client{
		Server: s, Hub: &FakeHub{S: s}, Events: &Recorder{}, Upstream: &FakeUpstream{}, Continuity: &MemContinuity{},
		Packages: &MemPackages{}, Repo: &MemAgreements{}, RuleStore: &MemRules{}, Cell: &app.Cell{}, RulesCell: &app.RulesCell{},
	}
	c.wire(t, seed, defaultAction, unknown)
	return c
}

// Restart rebuilds every component on the same stores, as after a process restart.
func (c *Client) Restart(t *testing.T, seed string) {
	t.Helper()
	c.Cell, c.RulesCell, c.Events = &app.Cell{}, &app.RulesCell{}, &Recorder{}
	c.wire(t, seed, "forward", "forward")
}

// wire assembles the use cases over the doubles.
func (c *Client) wire(t *testing.T, seed, defaultAction, unknown string) {
	t.Helper()
	s := c.Server
	hubDID := s.DID
	c.Trust = app.NewHubTrust(hubDID, s.Resolver, s.Clock, lifecycle.Defaults().SyncInterval, c.Events, NSIRI)
	c.Agreements = &app.Agreements{Repo: c.Repo, Events: c.Events, Clock: s.Clock, Namespace: NSIRI}
	ev := &evaluator_poc.Evaluator{Events: c.Events, Clock: s.Clock, Namespace: NSIRI}
	c.Audit = &app.Audit{Window: 100, Evaluator: ev, Events: c.Events, Clock: s.Clock, Namespace: NSIRI}
	c.Status = &app.Status{Cell: c.Cell, Rules: c.RulesCell, Trust: c.Trust, Packages: c.Packages, Events: c.Events, Clock: s.Clock, Namespace: NSIRI, HubDID: hubDID}
	c.Sync = &app.Sync{
		Hub: c.Hub, Trust: c.Trust, Continuity: c.Continuity, Packages: c.Packages, Agreements: c.Agreements, Audit: c.Audit,
		Status: c.Status, Cell: c.Cell, Clock: s.Clock, Events: c.Events, Namespace: NSIRI, HubDID: hubDID, Context: Context,
	}
	c.Rules = &app.Rules{Store: c.RuleStore, Cell: c.RulesCell, Events: c.Events, Clock: s.Clock, Namespace: NSIRI}
	if err := c.Rules.Init(seed); err != nil {
		t.Fatal(err)
	}
	c.Edge = &app.Edge{
		Explainer: app.Explainer{Snap: c.Cell, Rules: c.RulesCell, Evaluator: ev, Agreements: c.Repo, Clock: s.Clock, Settings: app.Settings{Namespace: NSIRI, DefaultAction: defaultAction, UnknownAgreement: unknown}},
		Upstream:  c.Upstream, Capture: c.Agreements, Audit: c.Audit, Events: c.Events, Metrics: noMetrics{},
	}
	c.GC = &app.GC{Packages: c.Packages, Agreements: c.Agreements, Cell: c.Cell, Events: c.Events, Clock: s.Clock, Namespace: NSIRI, Interval: lifecycle.Defaults().Freshness.TTL}
	if err := c.Sync.Restore(); err != nil {
		t.Fatal(err)
	}
}

// SyncNow runs one synchronization and fails the test on error.
func (c *Client) SyncNow(t *testing.T) {
	t.Helper()
	if err := c.Sync.Run(); err != nil {
		t.Fatalf("sync: %v", err)
	}
}

// SetMode changes the mode of a rule through the CRUD use case.
func (c *Client) SetMode(t *testing.T, id string, m domain.Mode) {
	t.Helper()
	rs := c.Rules.Get()
	r, ok := rs.Find(id)
	if !ok {
		t.Fatalf("rule %s not found", id)
	}
	r.Mode = m
	b, _ := json.Marshal(r)
	if _, err := c.Rules.Update(app.ETag(rs.Revision), id, b); err != nil {
		t.Fatal(err)
	}
}

// Send handles a message on a listener.
func (c *Client) Send(l domain.Listener, method, path string, body []byte) *app.Response {
	return c.Edge.Handle(&app.Request{Listener: l, Method: method, Path: path, Header: http.Header{"Content-Type": {"application/json"}}, Body: body})
}

// SeedPath returns the absolute path of the Annex C rule set.
func SeedPath(t *testing.T) string {
	t.Helper()
	return repoFile(t, "spec/examples/rules.json")
}

// ContractRequest is an F6 ContractRequestMessage using action.
func ContractRequest(action string) []byte {
	return []byte(`{"@type":"ContractRequestMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","action":"` + action + `"}`)
}

// ContractRequestPath is the path of the enforce rule contract-request.
const ContractRequestPath = "/negotiations/urn:uuid:p-1/request"

// AgreementMessage is the ContractAgreementMessage of agreement id with action and an optional declared profile.
func AgreementMessage(id, action, profile string) []byte {
	p := ""
	if profile != "" {
		p = `,"odrl:profile":"` + profile + `"`
	}
	return []byte(`{"@type":"ContractAgreementMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1","action":"` + action + `","agreement":{"@id":"` + id + `"` + p + `}}`)
}

// AgreementPath is the path of the contract-agreement rules.
const AgreementPath = "/negotiations/urn:uuid:c-1/agreement"

// TransferRequest is a TransferRequestMessage for an agreement.
func TransferRequest(agreementID string) []byte {
	return []byte(`{"@type":"TransferRequestMessage","consumerPid":"urn:uuid:c-1","agreementId":"` + agreementID + `"}`)
}

// VerdictOf returns the VHub-Verdict header of a response.
func VerdictOf(r *app.Response) string { return r.Header.Get(domain.HeaderName) }

// BodyCode returns the code member of a JSON body.
func BodyCode(b []byte) string {
	var v struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(b, &v)
	return v.Code
}

// Contains reports whether h contains s.
func Contains(h, s string) bool { return strings.Contains(h, s) }

// VC decodes a credential subject for assertions.
func VC[T any](t *testing.T, raw []byte) vc.Credential[T] {
	t.Helper()
	var c vc.Credential[T]
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c
}
