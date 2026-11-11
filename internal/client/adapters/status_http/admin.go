// Package status_http serves the admin API only on the admin listener: status, versions, rules CRUD and :explain, agreements, audit, health and metrics.
// Never mounted on DSP or management listeners; errors are problem+json (§4.12, REQ-CLI-019, REQ-CLI-020, REQ-CLI-026, REQ-CLI-028, REQ-CLI-032).
package status_http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/problem"
)

// Admin groups the use cases behind the admin API.
type Admin struct {
	Status     *app.Status
	Rules      *app.Rules
	Explainer  *app.Explainer
	Agreements *app.Agreements
	Audit      *app.Audit
	Cell       *app.Cell
	Metrics    *metrics.Registry
}

// Handler returns the admin mux.
func (a Admin) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, a.Status.Get()) })
	mux.HandleFunc("GET /versions", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, a.Status.Versions()) })
	mux.HandleFunc("GET /rules", func(w http.ResponseWriter, _ *http.Request) { a.ruleset(w, http.StatusOK, a.Rules.Get(), nil) })
	mux.HandleFunc("PUT /rules", func(w http.ResponseWriter, r *http.Request) {
		rs, err := a.Rules.Replace(r.Header.Get("If-Match"), body(r))
		a.ruleset(w, http.StatusOK, rs, err)
	})
	mux.HandleFunc("POST /rules", func(w http.ResponseWriter, r *http.Request) {
		rs, err := a.Rules.Add(r.Header.Get("If-Match"), body(r))
		a.ruleset(w, http.StatusCreated, rs, err)
	})
	mux.HandleFunc("POST /rules:explain", a.explain)
	mux.HandleFunc("GET /rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		rule, err := a.Rules.GetRule(r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("ETag", app.ETag(a.Rules.Get().Revision))
		writeJSON(w, http.StatusOK, rule)
	})
	mux.HandleFunc("PUT /rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		rs, err := a.Rules.Update(r.Header.Get("If-Match"), r.PathValue("id"), body(r))
		a.ruleset(w, http.StatusOK, rs, err)
	})
	mux.HandleFunc("DELETE /rules/{id}", func(w http.ResponseWriter, r *http.Request) {
		rs, err := a.Rules.Delete(r.Header.Get("If-Match"), r.PathValue("id"))
		if err != nil {
			writeError(w, err)
			return
		}
		w.Header().Set("ETag", app.ETag(rs.Revision))
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /agreements/{id...}", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.Agreements.Get(r.PathValue("id"), a.Cell.Load(), a.Status.Clock.Now())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, v)
	})
	mux.HandleFunc("GET /audit/reports/latest", func(w http.ResponseWriter, _ *http.Request) {
		rep := a.Audit.Latest()
		if rep == nil {
			writeError(w, codes.New(codes.APINotFound, "no audit report yet"))
			return
		}
		writeJSON(w, http.StatusOK, rep)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, _ *http.Request) {
		switch a.Status.Get().State {
		case domain.Active, domain.Transition, domain.Degraded:
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", metrics.ContentType)
		age := int64(-1)
		if snap := a.Cell.Load(); snap != nil {
			age = int64(a.Status.Clock.Now().Sub(snap.ValidFrom))
		}
		a.Metrics.Write(w, age)
	})
	return mux
}

// explainRequest is the body of POST /rules:explain.
type explainRequest struct {
	Listener domain.Listener `json:"listener"`
	Method   string          `json:"method"`
	Path     string          `json:"path"`
	Body     json.RawMessage `json:"body"`
}

// explain answers what the hot path would decide, without effects.
func (a Admin) explain(w http.ResponseWriter, r *http.Request) {
	var in explainRequest
	if err := json.Unmarshal(body(r), &in); err != nil {
		writeProblem(w, problem.Problem{Type: "about:blank", Title: "bad request", Status: http.StatusBadRequest, Detail: err.Error(), Severity: "error"})
		return
	}
	msg := []byte(in.Body)
	var s string
	if json.Unmarshal(in.Body, &s) == nil {
		msg = []byte(s)
	}
	writeJSON(w, http.StatusOK, a.Explainer.Explain(&app.Request{Listener: in.Listener, Method: in.Method, Path: in.Path, Body: msg}))
}

// ruleset writes a rule set with its revision as ETag.
func (a Admin) ruleset(w http.ResponseWriter, status int, rs *domain.Ruleset, err error) {
	if err != nil {
		writeError(w, err)
		return
	}
	w.Header().Set("ETag", app.ETag(rs.Revision))
	writeJSON(w, status, rs)
}

// body reads a request body.
func body(r *http.Request) []byte { b, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20)); return b }

// writeJSON writes a JSON body.
func writeJSON(w http.ResponseWriter, status int, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(b)
}

// writeProblem writes a problem document.
func writeProblem(w http.ResponseWriter, p problem.Problem) {
	w.Header().Set("Content-Type", problem.ContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(p.JSON()) // #nosec G705 -- problem+json body, never rendered as HTML.
}

// status maps a code to its admin API status.
func status(c codes.Code) int {
	switch c {
	case codes.APINotFound:
		return http.StatusNotFound
	case codes.APIPreconditionRequired:
		return http.StatusPreconditionRequired
	case codes.APIPreconditionFailed:
		return http.StatusPreconditionFailed
	}
	return http.StatusUnprocessableEntity
}

// writeError maps coded errors to problem+json.
func writeError(w http.ResponseWriter, err error) {
	if ce, ok := codes.As(err); ok {
		writeProblem(w, problem.From(ce, status(ce.Code)))
		return
	}
	writeProblem(w, problem.Problem{Type: "about:blank", Title: "internal error", Status: http.StatusInternalServerError, Detail: err.Error(), Severity: "error"})
}
