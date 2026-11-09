// Admin API served only on the admin listener: /status, /versions, /agreements, /audit, /healthz, /readyz and /metrics.
// Never mounted on DSP or management listeners; errors are problem+json (§4.11, REQ-CLI-019, REQ-CLI-020, REQ-CLI-026).
package status_http

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/ging/alexandria/internal/client/adapters/metrics"
	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/problem"
)

// Admin groups the use cases behind the admin API.
type Admin struct {
	Status     *app.Status
	Agreements *app.Agreements
	Audit      *app.Audit
	Cell       *app.Cell
	Metrics    *metrics.Registry
}

// Handler returns the admin mux.
func (a Admin) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.Status.Report()) })
	mux.HandleFunc("GET /versions", func(w http.ResponseWriter, r *http.Request) { writeJSON(w, a.Status.Versions()) })
	mux.HandleFunc("GET /agreements/{id...}", func(w http.ResponseWriter, r *http.Request) {
		v, err := a.Agreements.Get(r.PathValue("id"), a.Cell.Load(), a.Status.Clock.Now())
		if err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, v)
	})
	mux.HandleFunc("POST /agreements:import", func(w http.ResponseWriter, r *http.Request) {
		var pairs []app.ImportPair
		b, _ := io.ReadAll(io.LimitReader(r.Body, 8<<20))
		if err := json.Unmarshal(b, &pairs); err != nil {
			p := problem.Problem{Type: "about:blank", Title: "bad request", Status: http.StatusBadRequest, Detail: err.Error(), Severity: "error"}
			writeProblem(w, p)
			return
		}
		if err := a.Agreements.Import(pairs, a.Cell.Load()); err != nil {
			writeError(w, err)
			return
		}
		writeJSON(w, map[string]int{"imported": len(pairs)})
	})
	mux.HandleFunc("GET /audit/reports/latest", func(w http.ResponseWriter, r *http.Request) {
		rep := a.Audit.Latest()
		if rep == nil {
			writeError(w, codes.New(codes.ApiNotFound, "no audit report yet"))
			return
		}
		writeJSON(w, rep)
	})
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if a.Status.Ready() {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", metrics.ContentType)
		a.Metrics.Write(w, a.Status.ListingAge())
	})
	return mux
}

// writeJSON writes a JSON body.
func writeJSON(w http.ResponseWriter, v any) {
	b, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(b)
}

// writeProblem writes a problem document.
func writeProblem(w http.ResponseWriter, p problem.Problem) {
	w.Header().Set("Content-Type", problem.ContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(p.JSON())
}

// writeError maps coded errors to problem+json (API_NOT_FOUND is 404).
func writeError(w http.ResponseWriter, err error) {
	if ce, ok := codes.As(err); ok {
		st := http.StatusInternalServerError
		if ce.Code == codes.ApiNotFound {
			st = http.StatusNotFound
		}
		writeProblem(w, problem.From(ce, st))
		return
	}
	writeProblem(w, problem.Problem{Type: "about:blank", Title: "internal error", Status: 500, Detail: err.Error(), Severity: "error"})
}
