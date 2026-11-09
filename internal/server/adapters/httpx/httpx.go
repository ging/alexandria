// Shared HTTP helpers of the server driving adapters: problem+json responses, status mapping and the Date header.
// The Date header comes from the hub Clock so clients can estimate skew even under file_clock (REQ-HUB-017, REQ-HUB-019, REQ-CLI-025).
package httpx

import (
	"errors"
	"net/http"
	"strings"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/problem"
	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/server/app"
)

// Status maps a code to its HTTP status.
func Status(c codes.Code) int {
	switch {
	case c == codes.ApiForbiddenRole:
		return http.StatusForbidden
	case c == codes.ApiPreconditionRequired:
		return http.StatusPreconditionRequired
	case c == codes.ApiPreconditionFailed:
		return http.StatusPreconditionFailed
	case c == codes.ApiNotFound:
		return http.StatusNotFound
	case c == codes.ApiInvalidState:
		return http.StatusConflict
	case c == codes.ReleaseAborted:
		return http.StatusServiceUnavailable
	case strings.HasPrefix(string(c), "DATES_"), strings.HasPrefix(string(c), "LIFECYCLE_"), strings.HasPrefix(string(c), "POC_"):
		return http.StatusUnprocessableEntity
	}
	return http.StatusInternalServerError
}

// WriteProblem writes a problem document with the given status.
func WriteProblem(w http.ResponseWriter, p problem.Problem) {
	w.Header().Set("Content-Type", problem.ContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(p.JSON())
}

// WriteError maps an application error to problem+json.
func WriteError(w http.ResponseWriter, err error) {
	var f app.Findings
	if errors.As(err, &f) {
		WriteProblem(w, problem.FromAll(f, Status(f[0].Code)))
		return
	}
	if ce, ok := codes.As(err); ok {
		WriteProblem(w, problem.From(ce, Status(ce.Code)))
		return
	}
	WriteProblem(w, problem.Problem{Type: "about:blank", Title: "internal error", Status: http.StatusInternalServerError, Detail: err.Error(), Severity: "error"})
}

// BadRequest writes a 400 problem for an unreadable request body (no catalog code applies).
func BadRequest(w http.ResponseWriter, detail string) {
	WriteProblem(w, problem.Problem{Type: "about:blank", Title: "bad request", Status: http.StatusBadRequest, Detail: detail, Severity: "error"})
}

// WithDate sets the Date header from the hub clock on every response.
func WithDate(c timex.Clock, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", c.Now().Time().Format(http.TimeFormat))
		next.ServeHTTP(w, r)
	})
}

// JSON writes a JSON body with a status.
func JSON(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
