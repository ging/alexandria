// Package http_v1 is the driving adapter for the /v1 management API: bearer authentication, routing and compact JSON-LD projections.
// It holds no business rules: use cases authorize and validate, and coded errors become problem+json with @context (§3.9, REQ-HUB-012, REQ-HUB-017, REQ-HUB-029).
package http_v1

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
	"github.com/ging/alexandria/internal/telemetry"
)

// LDJSON is the media type of every 2xx body of /v1.
const LDJSON = "application/ld+json"

// TokenVerifier validates a bearer token (A1–A5) and returns its principal.
type TokenVerifier interface {
	Verify(raw string) (domain.Principal, error)
}

// Handler serves /v1.
type Handler struct {
	Hub     *app.Hub
	Tokens  TokenVerifier
	Context string
	Base    string
	MaxBody int64
	Logger  *slog.Logger
}

// handlerFunc is a /v1 handler receiving the optional principal.
type handlerFunc func(w http.ResponseWriter, r *http.Request, p *domain.Principal)

// Register mounts the /v1 routes on mux.
func (h Handler) Register(mux *http.ServeMux) {
	routes := map[string]handlerFunc{
		"POST /v1/namespaces":                                 h.createNamespace,
		"GET /v1/namespaces":                                  h.listNamespaces,
		"GET /v1/namespaces/{ns}":                             h.getNamespace,
		"GET /v1/namespaces/{ns}/stewards":                    h.listStewards,
		"POST /v1/namespaces/{ns}/stewards":                   h.assignSteward,
		"DELETE /v1/namespaces/{ns}/stewards/{id}":            h.removeSteward,
		"GET /v1/events":                                      h.events,
		"GET /v1/namespaces/{ns}/events":                      h.events,
		"POST /v1/namespaces/{ns}/vocabularies":               h.createVocabulary,
		"GET /v1/namespaces/{ns}/vocabularies":                h.listVocabularies,
		"GET /v1/namespaces/{ns}/vocabularies/{name}":         h.getVocabulary,
		"POST /v1/namespaces/{ns}/vocabularies/{name}":        h.vocabularyAction,
		"PUT /v1/namespaces/{ns}/vocabularies/{name}/content": h.putContent,
		"POST /v1/namespaces/{ns}/drafts":                     h.createDraft,
		"GET /v1/namespaces/{ns}/drafts/{id}":                 h.getDraft,
		"PUT /v1/namespaces/{ns}/drafts/{id}":                 h.replaceDraft,
		"POST /v1/namespaces/{ns}/drafts/{id}":                h.draftAction,
		"GET /v1/namespaces/{ns}/candidates/{id}":             h.getCandidate,
		"POST /v1/namespaces/{ns}/candidates/{id}":            h.candidateAction,
		"GET /v1/namespaces/{ns}/versions":                    h.listing,
		"GET /v1/namespaces/{ns}/versions/{version}":          h.record,
		"POST /v1/namespaces/{ns}/versions/{version}":         h.versionAction,
	}
	for pattern, f := range routes {
		mux.Handle(pattern, h.authenticated(f))
	}
	mux.HandleFunc("/v1/", func(w http.ResponseWriter, _ *http.Request) {
		h.problems().Error(w, codes.New(codes.APINotFound, "no such resource"))
	})
}

// problems returns the problem writer of /v1.
func (h Handler) problems() httpx.Problems {
	return httpx.Problems{Context: h.Context, Logger: h.Logger}
}

// authenticated validates a present bearer token (401 invalid_token on failure) and passes the principal, nil when absent.
func (h Handler) authenticated(next handlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := r.Header.Get("Authorization")
		if auth == "" {
			next(w, r, nil)
			return
		}
		raw, ok := strings.CutPrefix(auth, "Bearer ")
		p, err := h.Tokens.Verify(strings.TrimSpace(raw))
		if !ok || err != nil {
			logx.Default(h.Logger).WarnContext(r.Context(), "bearer authentication failed", slog.String("url.path", telemetry.Clean(r.URL.Path)), logx.Err(err))
			w.Header().Set("WWW-Authenticate", `Bearer error="invalid_token"`)
			h.problems().Error(w, codes.New(codes.APIUnauthenticated, "invalid bearer token"))
			return
		}
		next(w, r, &p)
	})
}

// action splits "{id}:{action}" from a path value.
func action(v string) (string, string) { id, a, _ := strings.Cut(v, ":"); return id, a }

// read decodes a JSON body into v; an empty body leaves v unchanged.
func (h Handler) read(w http.ResponseWriter, r *http.Request, v any) bool {
	b, err := h.body(r)
	if err != nil {
		h.problems().BadRequest(w, err.Error())
		return false
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return true
	}
	if err := json.Unmarshal(b, v); err != nil {
		h.problems().BadRequest(w, "body is not valid JSON: "+err.Error())
		return false
	}
	return true
}

// body reads the request body up to MaxBody+1 bytes.
func (h Handler) body(r *http.Request) ([]byte, error) {
	limit := h.MaxBody
	if limit == 0 {
		limit = 16 << 20
	}
	return io.ReadAll(io.LimitReader(r.Body, limit+1))
}

// out writes a JSON-LD document with its status.
func (h Handler) out(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		h.problems().Error(w, err)
		return
	}
	b, _ := json.Marshal(v)
	httpx.Body(w, status, LDJSON, b)
}

// nsIRI returns the IRI of a namespace.
func (h Handler) nsIRI(slug string) string { return h.Base + "/v1/namespaces/" + slug }

// createNamespace handles POST /v1/namespaces.
func (h Handler) createNamespace(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	var in app.NamespaceInput
	if !h.read(w, r, &in) {
		return
	}
	n, err := h.Hub.Namespaces.Create(p, in)
	if err == nil {
		w.Header().Set("Location", n.ID)
	}
	h.out(w, http.StatusCreated, h.doc(h.namespace(n)), err)
}

// listNamespaces handles GET /v1/namespaces.
func (h Handler) listNamespaces(w http.ResponseWriter, _ *http.Request, _ *domain.Principal) {
	ns, err := h.Hub.Namespaces.List()
	items := make([]any, 0, len(ns))
	for _, n := range ns {
		items = append(items, h.namespace(n))
	}
	h.out(w, http.StatusOK, h.graph(items), err)
}

// getNamespace handles GET /v1/namespaces/{ns}.
func (h Handler) getNamespace(w http.ResponseWriter, r *http.Request, _ *domain.Principal) {
	n, err := h.Hub.Namespaces.Get(r.PathValue("ns"))
	h.out(w, http.StatusOK, h.doc(h.namespace(n)), err)
}

// listStewards handles GET .../stewards.
func (h Handler) listStewards(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	ns := r.PathValue("ns")
	ss, err := h.Hub.Namespaces.ListStewards(p, ns)
	items := make([]any, 0, len(ss))
	for _, s := range ss {
		items = append(items, h.steward(ns, s))
	}
	h.out(w, http.StatusOK, h.graph(items), err)
}

// assignSteward handles POST .../stewards.
func (h Handler) assignSteward(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	var in app.StewardInput
	if !h.read(w, r, &in) {
		return
	}
	ns := r.PathValue("ns")
	s, err := h.Hub.Namespaces.AssignSteward(p, ns, in)
	h.out(w, http.StatusCreated, h.doc(h.steward(ns, s)), err)
}

// removeSteward handles DELETE .../stewards/{id}.
func (h Handler) removeSteward(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	if err := h.Hub.Namespaces.RemoveSteward(p, r.PathValue("ns"), r.PathValue("id")); err != nil {
		h.problems().Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// events handles GET /v1/events and GET .../events, paged by after (an id or an event IRI).
func (h Handler) events(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	after := r.URL.Query().Get("after")
	after = after[strings.LastIndex(after, "/")+1:]
	n, _ := strconv.ParseInt(after, 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	ns := r.PathValue("ns")
	evs, err := h.Hub.Query.Events(p, ns, n, limit)
	items := make([]any, 0, len(evs))
	for _, e := range evs {
		items = append(items, h.event(e))
	}
	h.out(w, http.StatusOK, h.graph(items), err)
}

// createVocabulary handles POST .../vocabularies.
func (h Handler) createVocabulary(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	var in app.VocabInput
	if !h.read(w, r, &in) {
		return
	}
	ns := r.PathValue("ns")
	v, err := h.Hub.Vocabularies.Create(p, ns, in)
	h.out(w, http.StatusCreated, h.doc(h.vocabulary(ns, v)), err)
}

// listVocabularies handles GET .../vocabularies.
func (h Handler) listVocabularies(w http.ResponseWriter, r *http.Request, _ *domain.Principal) {
	ns := r.PathValue("ns")
	vs, err := h.Hub.VocabReader.List(ns)
	items := make([]any, 0, len(vs))
	for _, v := range vs {
		items = append(items, h.vocabulary(ns, v))
	}
	h.out(w, http.StatusOK, h.graph(items), err)
}

// getVocabulary handles GET .../vocabularies/{name}.
func (h Handler) getVocabulary(w http.ResponseWriter, r *http.Request, _ *domain.Principal) {
	ns := r.PathValue("ns")
	v, err := h.Hub.VocabReader.Get(ns, r.PathValue("name"))
	h.out(w, http.StatusOK, h.doc(h.vocabulary(ns, v)), err)
}

// vocabularyAction handles POST .../vocabularies/{name}:refresh.
func (h Handler) vocabularyAction(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	name, act := action(r.PathValue("name"))
	if act != "refresh" {
		h.problems().Error(w, codes.New(codes.APINotFound, "no such action"))
		return
	}
	ns := r.PathValue("ns")
	rev, created, err := h.Hub.Vocabularies.Refresh(p, ns, name)
	h.out(w, revisionStatus(created), h.doc(h.revision(ns, name, rev)), err)
}

// putContent handles PUT .../vocabularies/{name}/content.
func (h Handler) putContent(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	b, err := h.body(r)
	if err != nil {
		h.problems().BadRequest(w, err.Error())
		return
	}
	ns, name := r.PathValue("ns"), r.PathValue("name")
	rev, created, err := h.Hub.Vocabularies.PutContent(p, ns, name, r.Header.Get("Content-Type"), r.Header.Get("If-Match"), b)
	if err == nil {
		w.Header().Set("ETag", `"`+rev.Digest.Hex()+`"`)
	}
	h.out(w, revisionStatus(created), h.doc(h.revision(ns, name, rev)), err)
}

// revisionStatus is 201 for a new revision and 200 for unchanged bytes.
func revisionStatus(created bool) int {
	if created {
		return http.StatusCreated
	}
	return http.StatusOK
}

// createDraft handles POST .../drafts.
func (h Handler) createDraft(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	b, err := h.body(r)
	if err != nil {
		h.problems().BadRequest(w, err.Error())
		return
	}
	ns := r.PathValue("ns")
	d, err := h.Hub.Authoring.CreateDraft(p, ns, b)
	if err == nil {
		w.Header().Set("Location", h.nsIRI(ns)+"/drafts/"+d.ID)
	}
	h.draftOut(w, http.StatusCreated, d, err)
}

// getDraft handles GET .../drafts/{id}.
func (h Handler) getDraft(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	d, err := h.Hub.Authoring.GetDraft(p, r.PathValue("ns"), r.PathValue("id"))
	h.draftOut(w, http.StatusOK, d, err)
}

// replaceDraft handles PUT .../drafts/{id}.
func (h Handler) replaceDraft(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	b, err := h.body(r)
	if err != nil {
		h.problems().BadRequest(w, err.Error())
		return
	}
	d, err := h.Hub.Authoring.ReplaceDraft(p, r.PathValue("ns"), r.PathValue("id"), r.Header.Get("If-Match"), b)
	h.draftOut(w, http.StatusOK, d, err)
}

// draftOut writes a draft with its ETag.
func (h Handler) draftOut(w http.ResponseWriter, status int, d domain.Draft, err error) {
	if err == nil {
		w.Header().Set("ETag", d.ETag)
	}
	doc := map[string]any{"id": h.nsIRI(d.Namespace) + "/drafts/" + d.ID, "namespace": h.nsIRI(d.Namespace), "rev": d.Rev}
	if json.Valid(d.Doc) {
		doc["document"] = json.RawMessage(d.Doc)
	}
	h.out(w, status, h.doc(doc), err)
}

// draftAction handles POST .../drafts/{id}:validate and :propose.
func (h Handler) draftAction(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	ns := r.PathValue("ns")
	id, act := action(r.PathValue("id"))
	switch act {
	case "validate":
		rep, err := h.Hub.Authoring.Validate(p, ns, id)
		h.out(w, http.StatusOK, h.doc(map[string]any{"valid": rep.Valid, "class": rep.Class, "findings": rep.Findings}), err)
	case "propose":
		c, err := h.Hub.Authoring.Propose(p, ns, id)
		h.out(w, http.StatusCreated, h.doc(h.candidate(c)), err)
	default:
		h.problems().Error(w, codes.New(codes.APINotFound, "no such action"))
	}
}

// getCandidate handles GET .../candidates/{id}.
func (h Handler) getCandidate(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	c, err := h.Hub.Query.Candidate(p, r.PathValue("ns"), r.PathValue("id"))
	h.out(w, http.StatusOK, h.doc(h.candidate(c)), err)
}

// candidateAction handles POST .../candidates/{id}:approve and :reject.
func (h Handler) candidateAction(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	ns := r.PathValue("ns")
	id, act := action(r.PathValue("id"))
	switch act {
	case "approve":
		var o app.ApproveOptions
		if !h.read(w, r, &o) {
			return
		}
		rec, err := h.Hub.Release.Approve(p, ns, id, o)
		h.credential(w, http.StatusCreated, rec, err)
	case "reject":
		var o struct {
			Comment string `json:"comment"`
		}
		if !h.read(w, r, &o) {
			return
		}
		c, err := h.Hub.Release.Reject(p, ns, id, o.Comment)
		h.out(w, http.StatusOK, h.doc(h.candidate(c)), err)
	default:
		h.problems().Error(w, codes.New(codes.APINotFound, "no such action"))
	}
}

// listing handles GET .../versions (public, no-cache).
func (h Handler) listing(w http.ResponseWriter, r *http.Request, _ *domain.Principal) {
	b, err := h.Hub.Listing.Latest(r.PathValue("ns"))
	w.Header().Set("Cache-Control", "no-cache")
	h.credential(w, http.StatusOK, b, err)
}

// record handles GET .../versions/{version} (public, no-cache).
func (h Handler) record(w http.ResponseWriter, r *http.Request, _ *domain.Principal) {
	b, err := h.Hub.Listing.Record(r.PathValue("ns"), r.PathValue("version"))
	w.Header().Set("Cache-Control", "no-cache")
	h.credential(w, http.StatusOK, b, err)
}

// versionAction handles POST .../versions/{version}:revoke.
func (h Handler) versionAction(w http.ResponseWriter, r *http.Request, p *domain.Principal) {
	v, act := action(r.PathValue("version"))
	if act != "revoke" {
		h.problems().Error(w, codes.New(codes.APINotFound, "no such action"))
		return
	}
	var o app.RevokeOptions
	if !h.read(w, r, &o) {
		return
	}
	b, err := h.Hub.Revocation.Revoke(p, r.PathValue("ns"), v, o)
	h.credential(w, http.StatusOK, b, err)
}

// credential writes a signed credential exactly as stored.
func (h Handler) credential(w http.ResponseWriter, status int, b []byte, err error) {
	if err != nil {
		h.problems().Error(w, err)
		return
	}
	httpx.Body(w, status, LDJSON, b)
}
