// Driving adapter for the /v1 management API: translates HTTP into use-case calls and coded errors into problem+json.
// Holds no business rules; authentication is the auth_static middleware (REQ-HUB-012, REQ-HUB-017, REQ-HUB-018, REQ-HUB-019).
package http_v1

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/problem"
	"github.com/ging/alexandria/internal/server/adapters/httpx"
	"github.com/ging/alexandria/internal/server/app"
	"github.com/ging/alexandria/internal/server/domain"
)

// Authenticator resolves the principal of a request.
type Authenticator interface {
	Authenticate(r *http.Request) (domain.Principal, bool)
}

// Handler serves /v1.
type Handler struct {
	Hub  *app.Hub
	Auth Authenticator
}

// Register mounts the /v1 routes on mux.
func (h Handler) Register(mux *http.ServeMux) {
	mux.Handle("/v1/", h.authenticated(http.HandlerFunc(h.route)))
}

// authenticated rejects requests without a known bearer token (401 with API_FORBIDDEN_ROLE, agent decision).
func (h Handler) authenticated(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := h.Auth.Authenticate(r); !ok {
			w.Header().Set("WWW-Authenticate", `Bearer realm="vhub"`)
			httpx.WriteProblem(w, problem.From(codes.New(codes.ApiForbiddenRole, "authentication required"), http.StatusUnauthorized))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// route dispatches on collection, id and :action.
func (h Handler) route(w http.ResponseWriter, r *http.Request) {
	p, _ := h.Auth.Authenticate(r)
	parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/v1/"), "/", 2)
	id, action := "", ""
	if len(parts) == 2 {
		id, action, _ = strings.Cut(parts[1], ":")
	}
	key := r.Method + " " + parts[0]
	if len(parts) == 2 {
		key += "/{id}"
		if action != "" {
			key += ":" + action
		}
	}
	switch key {
	case "POST drafts":
		h.createDraft(w, r, p)
	case "GET drafts/{id}":
		h.draftOut(w, http.StatusOK, true)(h.Hub.Authoring.GetDraft(p, id))
	case "PUT drafts/{id}":
		body, ok := read(w, r)
		if ok {
			h.draftOut(w, http.StatusOK, false)(h.Hub.Authoring.ReplaceDraft(p, id, r.Header.Get("If-Match"), body))
		}
	case "POST drafts/{id}:validate":
		rep, err := h.Hub.Authoring.Validate(p, id)
		writeJSON(w, http.StatusOK, rep, err)
	case "POST drafts/{id}:propose":
		c, err := h.Hub.Authoring.Propose(p, id)
		writeJSON(w, http.StatusCreated, c, err)
	case "GET candidates/{id}":
		c, err := h.Hub.Query.Candidate(p, id)
		writeJSON(w, http.StatusOK, c, err)
	case "POST candidates/{id}:approve":
		var o app.ApproveOptions
		if decode(w, r, &o) {
			rec, err := h.Hub.Release.Approve(p, id, o)
			writeRaw(w, http.StatusCreated, rec, err, false)
		}
	case "POST candidates/{id}:reject":
		var o struct {
			Comment string `json:"comment"`
		}
		if decode(w, r, &o) {
			c, err := h.Hub.Release.Reject(p, id, o.Comment)
			writeJSON(w, http.StatusOK, c, err)
		}
	case "GET versions":
		l, err := h.Hub.Listing.Latest(p)
		writeRaw(w, http.StatusOK, l, err, true)
	case "GET versions/{id}":
		rec, err := h.Hub.Listing.Record(p, id)
		writeRaw(w, http.StatusOK, rec, err, true)
	case "POST versions/{id}:revoke":
		var o app.RevokeOptions
		if decode(w, r, &o) {
			rec, err := h.Hub.Revocation.Revoke(p, id, o)
			writeRaw(w, http.StatusOK, rec, err, false)
		}
	case "GET events":
		after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
		ev, err := h.Hub.Query.Events(p, after, 100)
		writeJSON(w, http.StatusOK, map[string]any{"events": ev}, err)
	default:
		httpx.WriteError(w, codes.New(codes.ApiNotFound, "%s %s is not a /v1 resource", r.Method, r.URL.Path))
	}
}

// createDraft handles POST /v1/drafts.
func (h Handler) createDraft(w http.ResponseWriter, r *http.Request, p domain.Principal) {
	body, ok := read(w, r)
	if !ok {
		return
	}
	d, err := h.Hub.Authoring.CreateDraft(p, body)
	if err == nil {
		w.Header().Set("Location", "/v1/drafts/"+d.ID)
	}
	h.draftOut(w, http.StatusCreated, false)(d, err)
}

// draftOut writes a draft with its ETag, either as its document or as {id, rev}.
func (h Handler) draftOut(w http.ResponseWriter, status int, asDoc bool) func(domain.Draft, error) {
	return func(d domain.Draft, err error) {
		if err != nil {
			httpx.WriteError(w, err)
			return
		}
		w.Header().Set("ETag", d.ETag)
		if asDoc {
			httpx.JSON(w, status, d.Doc)
			return
		}
		b, _ := json.Marshal(d)
		httpx.JSON(w, status, b)
	}
}

// read returns the request body.
func read(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	b, err := io.ReadAll(io.LimitReader(r.Body, 8<<20))
	if err != nil {
		httpx.BadRequest(w, err.Error())
		return nil, false
	}
	return b, true
}

// decode reads an optional JSON body into v.
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	b, ok := read(w, r)
	if !ok {
		return false
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return true
	}
	if err := json.Unmarshal(b, v); err != nil {
		httpx.BadRequest(w, err.Error())
		return false
	}
	return true
}

// writeJSON writes v or the error.
func writeJSON(w http.ResponseWriter, status int, v any, err error) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	b, _ := json.Marshal(v)
	httpx.JSON(w, status, b)
}

// writeRaw writes signed bytes as served, with private no-cache for the versions resources.
func writeRaw(w http.ResponseWriter, status int, b []byte, err error, versions bool) {
	if err != nil {
		httpx.WriteError(w, err)
		return
	}
	if versions {
		w.Header().Set("Cache-Control", "private, no-cache")
	}
	httpx.JSON(w, status, b)
}
