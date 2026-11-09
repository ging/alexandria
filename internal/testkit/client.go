// Package testkit provides an in-process alexandria-client wired to an in-memory hub through fake HubAPI and DIDResolver adapters with fault injection.
// Lets client tests replay F6 and corrupt listings, packages and DID documents without network (F8 §8.7).
package testkit

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/ging/alexandria/internal/client/adapters/evaluator_poc"
	"github.com/ging/alexandria/internal/client/adapters/receipt_scitt_fake"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/did"
	"github.com/ging/alexandria/internal/core/ni"
	"github.com/ging/alexandria/internal/core/pkgv"
	"github.com/ging/alexandria/internal/core/timex"
)

// FakeHub serves the in-memory hub through the HubAPI port.
type FakeHub struct {
	S            *Server
	mu           sync.Mutex
	Down         bool
	Unauthorized bool
	Calls        int
	Date         *timex.Instant
	Tamper       func([]byte) []byte
	TamperPkg    func(ni.Digest, *pkgv.Files)
}

// Listing returns the hub listing, applying injected faults.
func (h *FakeHub) Listing() (app.ListingResponse, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Calls++
	switch {
	case h.Down:
		return app.ListingResponse{}, app.ErrUnreachable
	case h.Unauthorized:
		return app.ListingResponse{}, app.ErrUnauthorized
	}
	b, err := h.S.Hub.Listing.Latest(Member)
	if err != nil {
		return app.ListingResponse{}, app.ErrUnreachable
	}
	if h.Tamper != nil {
		b = h.Tamper(append([]byte(nil), b...))
	}
	date := h.S.Clock.Now()
	if h.Date != nil {
		date = *h.Date
	}
	return app.ListingResponse{Body: b, Date: date, HasDate: true}, nil
}

// Package downloads a package from /pub, applying injected faults.
func (h *FakeHub) Package(d ni.Digest) (pkgv.Files, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.Calls++
	if h.Down {
		return pkgv.Files{}, app.ErrUnreachable
	}
	pub := h.S.Hub.Publication
	var f pkgv.Files
	var err error
	for name, dst := range map[string]*[]byte{"manifest.json": &f.Manifest, "manifest.sig": &f.Sig, "manifest.receipt": &f.Receipt, "space-lock.json": &f.SpaceLock} {
		if *dst, err = pub.PackageFile(d.Segment(), name); err != nil {
			return pkgv.Files{}, err
		}
	}
	var m pkgv.Manifest
	_ = json.Unmarshal(f.Manifest, &m)
	f.Content = map[string][]byte{}
	for _, e := range m.Files {
		if f.Content[e.Path], err = pub.Blob(e.Digest.Hex()); err != nil {
			return pkgv.Files{}, err
		}
	}
	if h.TamperPkg != nil {
		h.TamperPkg(d, &f)
	}
	return f, nil
}

// FakeResolver resolves the hub DID from the in-memory signer.
type FakeResolver struct {
	S     *Server
	mu    sync.Mutex
	Down  bool
	Calls int
}

// Resolve returns the current DID document of the signer.
func (r *FakeResolver) Resolve(did.DID) (did.Document, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.Calls++
	if r.Down {
		return did.Document{}, errors.New("DID document unreachable")
	}
	return r.S.Signer.Document(), nil
}

// Recorder is an EventSink that keeps every event.
type Recorder struct {
	mu     sync.Mutex
	Events []any
}

// Emit records an event.
func (r *Recorder) Emit(e any) { r.mu.Lock(); r.Events = append(r.Events, e); r.mu.Unlock() }

// Codes returns every code carried by recorded events, in order.
func (r *Recorder) Codes() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for _, e := range r.Events {
		switch v := e.(type) {
		case app.Verdict:
			out = append(out, v.Codes...)
		case app.Notice:
			if v.Code != "" {
				out = append(out, v.Code)
			}
		}
	}
	return out
}

// Has reports whether some event carries code.
func (r *Recorder) Has(code string) bool {
	for _, c := range r.Codes() {
		if c == code {
			return true
		}
	}
	return false
}

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

// Verdicts returns the vhub.verdict events.
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

// Reset forgets recorded events.
func (r *Recorder) Reset() { r.mu.Lock(); r.Events = nil; r.mu.Unlock() }

// FakeUpstream records forwarded requests and answers with a fixed status.
type FakeUpstream struct {
	mu       sync.Mutex
	Status   int
	Body     []byte
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
	body := u.Body
	if body == nil {
		body = []byte(`{"ok":true}`)
	}
	return &app.Response{Status: st, Header: http.Header{"Content-Type": {"application/json"}}, Body: body}, nil
}

// MemContinuity keeps continuity as JSON, as a file store would.
type MemContinuity struct {
	mu sync.Mutex
	b  []byte
}

// Load decodes the stored continuity.
func (m *MemContinuity) Load() (app.Continuity, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var c app.Continuity
	if m.b == nil {
		return c, nil
	}
	return c, json.Unmarshal(m.b, &c)
}

// Save encodes continuity.
func (m *MemContinuity) Save(c app.Continuity) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := json.Marshal(c)
	m.b = b
	return err
}

// MemPackages keeps package files by digest.
type MemPackages struct {
	mu sync.Mutex
	m  map[ni.Digest]pkgv.Files
}

// Get returns stored files.
func (p *MemPackages) Get(d ni.Digest) (pkgv.Files, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	f, ok := p.m[d]
	return f, ok, nil
}

// Put stores files.
func (p *MemPackages) Put(d ni.Digest, f pkgv.Files) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.m == nil {
		p.m = map[ni.Digest]pkgv.Files{}
	}
	p.m[d] = f
	return nil
}

// Retain drops packages not in keep.
func (p *MemPackages) Retain(keep []ni.Digest) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	k := map[ni.Digest]bool{}
	for _, d := range keep {
		k[d] = true
	}
	for d := range p.m {
		if !k[d] {
			delete(p.m, d)
		}
	}
	return nil
}

// MemAgreements is an in-memory agreement registry.
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

// Client is an in-process alexandria-client with its doubles exposed.
type Client struct {
	Hub        *FakeHub
	Resolver   *FakeResolver
	Events     *Recorder
	Upstream   *FakeUpstream
	Continuity *MemContinuity
	Packages   *MemPackages
	Repo       *MemAgreements
	Clock      timex.Clock
	Cell       *app.Cell
	Trust      *app.HubTrust
	Sync       *app.Sync
	Edge       *app.Edge
	Status     *app.Status
	Agreements *app.Agreements
	Audit      *app.Audit
}

// noMetrics discards classification counts.
type noMetrics struct{}

// Classified ignores the count.
func (noMetrics) Classified(string, string, string) {}

// NewClient builds a client on the server's clock with the given routes.
func NewClient(s *Server, routes []domain.Route) *Client {
	c := &Client{
		Hub: &FakeHub{S: s}, Resolver: &FakeResolver{S: s}, Events: &Recorder{}, Upstream: &FakeUpstream{},
		Continuity: &MemContinuity{}, Packages: &MemPackages{}, Repo: &MemAgreements{}, Clock: s.Clock,
	}
	c.wire(routes, "forward", "forward")
	return c
}

// wire builds the use cases over the doubles.
func (c *Client) wire(routes []domain.Route, defaultAction, unknown string) {
	c.Cell = &app.Cell{}
	c.Trust = app.NewHubTrust(HubDID, c.Resolver, c.Clock, 6*timex.Hour, c.Events, Space)
	ev := &evaluator_poc.Evaluator{Events: c.Events, Clock: c.Clock, Space: Space}
	c.Agreements = &app.Agreements{Repo: c.Repo, Events: c.Events, Clock: c.Clock, Space: Space}
	c.Audit = &app.Audit{Window: 100, Evaluator: ev, Events: c.Events, Clock: c.Clock, Space: Space}
	c.Status = &app.Status{Cell: c.Cell, Clock: c.Clock, Trust: c.Trust, Packages: c.Packages, Events: c.Events, Space: Space, HubDID: string(HubDID), Gates: map[domain.Gate]domain.Mode{}}
	c.Sync = &app.Sync{
		Hub: c.Hub, Trust: c.Trust, Receipts: receipt_scitt_fake.Verifier{}, Continuity: c.Continuity, Packages: c.Packages,
		Agreements: c.Agreements, Audit: c.Audit, Status: c.Status, Cell: c.Cell, Clock: c.Clock, Events: c.Events, Space: Space,
	}
	c.Edge = &app.Edge{
		Cell: c.Cell, Clock: c.Clock, Evaluator: ev, Upstream: c.Upstream, Agreements: c.Agreements, Audit: c.Audit,
		Events: c.Events, Metrics: noMetrics{}, Routes: routes, DefaultAction: defaultAction, UnknownAgreement: unknown, Space: Space,
	}
}

// Configure rebuilds the client with other policies, keeping its stores (simulates a restart).
func (c *Client) Configure(routes []domain.Route, defaultAction, unknown string) error {
	c.wire(routes, defaultAction, unknown)
	return c.Sync.Restore()
}

// Send posts a body to a route path on a listener.
func (c *Client) Send(l domain.Listener, method, path string, body []byte) *app.Response {
	return c.Edge.Handle(&app.Request{Listener: l, Method: method, Path: path, Header: http.Header{}, Body: body})
}

// F6Routes returns the routes of the F6 scenario in enforce.
func F6Routes() []domain.Route {
	sel := func(s string) *domain.Body { return &domain.Body{Select: s} }
	return []domain.Route{
		{
			Listener: domain.DSPIngress, Match: domain.Match{Method: "POST", Path: "/negotiations/request"}, Message: "ContractRequestMessage",
			Gates: []domain.Gate{domain.GateDSP, domain.GatePolicy}, Mode: domain.Enforce, Body: sel("$.offer.permission[0]"),
		},
		{
			Listener: domain.DSPEgress, Match: domain.Match{Method: "POST", Path: "/negotiations/:consumerPid/agreement"}, Message: "ContractAgreementMessage",
			Gates: []domain.Gate{domain.GateDSP, domain.GatePolicy}, Mode: domain.Enforce, Body: sel("$.agreement.permission[0]"),
		},
		{
			Listener: domain.DSPIngress, Match: domain.Match{Method: "POST", Path: "/transfers/request"}, Message: "TransferRequestMessage",
			Gates: []domain.Gate{domain.GateDSP, domain.GateAdmission}, Mode: domain.Enforce,
		},
	}
}

// ContractRequest builds a ContractRequestMessage whose offer uses action.
func ContractRequest(action string) []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld"],"@type":"ContractRequestMessage","consumerPid":"urn:uuid:c-1",` +
		`"offer":{"@id":"urn:uuid:offer-1","@type":"Offer","target":"urn:uuid:ds-1","permission":[{"action":"` + action + `"}]},"callbackAddress":"http://consumer/cb"}`)
}

// AgreementMessage builds a ContractAgreementMessage for agreement id using action.
func AgreementMessage(id, action, profile string) []byte {
	p := ""
	if profile != "" {
		p = `"odrl:profile":"` + profile + `",`
	}
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld"],"@type":"ContractAgreementMessage","providerPid":"urn:uuid:p-1","consumerPid":"urn:uuid:c-1",` +
		`"agreement":{"@id":"` + id + `","@type":"Agreement",` + p + `"target":"urn:uuid:ds-1","permission":[{"action":"` + action + `"}]},"callbackAddress":"http://provider/cb"}`)
}

// TransferRequest builds a TransferRequestMessage for an agreement.
func TransferRequest(agreementID string) []byte {
	return []byte(`{"@context":["https://w3id.org/dspace/2025/1/context.jsonld"],"@type":"TransferRequestMessage","consumerPid":"urn:uuid:t-1","agreementId":"` +
		agreementID + `","format":"HTTP_PULL","callbackAddress":"http://consumer/cb"}`)
}

// VerdictOf returns the VHub-Verdict header of a response.
func VerdictOf(r *app.Response) string { return r.Header.Get(domain.HeaderName) }

// BodyCode extracts "code" from a JSON error body.
func BodyCode(b []byte) string {
	var x struct{ Code string }
	_ = json.Unmarshal(b, &x)
	return x.Code
}

// Contains reports whether the header mentions s.
func Contains(h, s string) bool { return strings.Contains(h, s) }
