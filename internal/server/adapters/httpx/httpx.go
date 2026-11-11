// Package httpx provides the shared HTTP helpers of the server driving adapters: problem+json with @context, status mapping and the Date header.
// The Date header comes from the hub Clock so clients can estimate skew even under file_clock (REQ-HUB-017, REQ-HUB-019, REQ-HUB-029, REQ-CLI-025).
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
	switch c {
	case codes.APIUnauthenticated:
		return http.StatusUnauthorized
	case codes.APIForbiddenRole:
		return http.StatusForbidden
	case codes.APIPreconditionRequired:
		return http.StatusPreconditionRequired
	case codes.APIPreconditionFailed:
		return http.StatusPreconditionFailed
	case codes.APINotFound:
		return http.StatusNotFound
	case codes.APIInvalidState, codes.NSExists, codes.StewardExists, codes.VocabExists:
		return http.StatusConflict
	case codes.VocabTooLarge:
		return http.StatusRequestEntityTooLarge
	case codes.VocabUpstreamFailed:
		return http.StatusBadGateway
	case codes.ReleaseAborted, codes.SignerKeyNotPublished:
		return http.StatusServiceUnavailable
	}
	for _, p := range []string{"DATES_", "LIFECYCLE_", "POC_", "NS_", "VOCAB_"} {
		if strings.HasPrefix(string(c), p) {
			return http.StatusUnprocessableEntity
		}
	}
	return http.StatusInternalServerError
}

// Problems writes problem documents with a JSON-LD @context.
type Problems struct{ Context string }

// Write writes a problem document; 401 adds WWW-Authenticate (RFC 6750).
func (ps Problems) Write(w http.ResponseWriter, p problem.Problem) {
	if p.Status == http.StatusUnauthorized && w.Header().Get("WWW-Authenticate") == "" {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	w.Header().Set("Content-Type", problem.ContentType)
	w.WriteHeader(p.Status)
	_, _ = w.Write(p.WithContext(ps.Context).JSON())
}

// Error maps an application error to problem+json.
func (ps Problems) Error(w http.ResponseWriter, err error) {
	var f app.Findings
	if errors.As(err, &f) {
		ps.Write(w, problem.FromAll(f, http.StatusUnprocessableEntity))
		return
	}
	if ce, ok := codes.As(err); ok {
		ps.Write(w, problem.From(ce, Status(ce.Code)))
		return
	}
	ps.Write(w, problem.Problem{Type: "about:blank", Title: "internal error", Status: http.StatusInternalServerError, Detail: err.Error(), Severity: "error"})
}

// BadRequest writes a 400 problem for an unreadable request body (no catalog code applies).
func (ps Problems) BadRequest(w http.ResponseWriter, detail string) {
	ps.Write(w, problem.Problem{Type: "about:blank", Title: "bad request", Status: http.StatusBadRequest, Detail: detail, Severity: "error"})
}

// WithDate sets the Date header from the hub clock on every response.
func WithDate(c timex.Clock, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Date", c.Now().Time().Format(http.TimeFormat))
		next.ServeHTTP(w, r)
	})
}

// Body writes a body with a status and content type.
func Body(w http.ResponseWriter, status int, contentType string, body []byte) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}
