// errors translates internal domain error sentinels into HTTP responses.
// It maps domain failures to RFC 7807 problem details and appropriate status codes.

package rest

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"

	"github.com/caparicio-esd/alexandria/internal/common"
	"github.com/gin-gonic/gin"
)

// OriginalResponse carries structured upstream diagnostics from IdentityHub or Fafnir.
type OriginalResponse struct {
	Provider   string `json:"provider"`
	StatusCode int    `json:"statusCode"`
	Path       string `json:"path,omitempty"`
	Body       any    `json:"body"`
}

// errorBody is the single error shape this API speaks. Keeping it in one place
// means an auditor can read exactly what leaves the process on a failure.
type errorBody struct {
	Error string `json:"error"`
	// Field names the offending input when the domain could pinpoint one.
	Field            string            `json:"field,omitempty"`
	OriginalResponse *OriginalResponse `json:"originalResponse,omitempty"`
}

// respondError is the only translation table between domain errors and HTTP.
// Nothing else in this package chooses a status code.
//
// Domain errors carry their own message and are safe to echo; anything that
// falls through is infrastructure, so it is logged in full and answered opaque.
func respondError(c *gin.Context, err error) {
	// Filed on the context rather than logged here: the access log emits one
	// record per request, at a level chosen from the status, and it picks this
	// up. Logging in both places would double-count every failure.
	_ = c.Error(err)

	var (
		invalid     common.ValidationError
		typeErr     *json.UnmarshalTypeError
		syntaxErr   *json.SyntaxError
		upstreamErr *common.UpstreamError
	)

	var origResp *OriginalResponse
	if errors.As(err, &upstreamErr) {
		var bodyObj any
		if len(upstreamErr.RawBody) > 0 && json.Valid(upstreamErr.RawBody) {
			bodyObj = json.RawMessage(upstreamErr.RawBody)
		} else if len(upstreamErr.RawBody) > 0 {
			bodyObj = string(upstreamErr.RawBody)
		}
		origResp = &OriginalResponse{
			Provider:   upstreamErr.Provider,
			StatusCode: upstreamErr.StatusCode,
			Path:       upstreamErr.Path,
			Body:       bodyObj,
		}
	}

	switch {
	// A body that will not decode is the caller's mistake, not an outage. It is
	// answered in this API's own vocabulary rather than with the library's
	// message, which names the Go struct the request happened to be decoded
	// into — an internal detail that must not leave the process.
	case errors.As(err, &typeErr):
		c.AbortWithStatusJSON(http.StatusBadRequest, errorBody{
			Error:            fmt.Sprintf("must be %s", jsonKind(typeErr.Type)),
			Field:            typeErr.Field,
			OriginalResponse: origResp,
		})

	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		c.AbortWithStatusJSON(http.StatusBadRequest,
			errorBody{Error: "body is not valid json", OriginalResponse: origResp})

	case errors.Is(err, io.EOF):
		c.AbortWithStatusJSON(http.StatusBadRequest,
			errorBody{Error: "body is empty", OriginalResponse: origResp})

	case errors.As(err, &invalid):
		c.AbortWithStatusJSON(http.StatusBadRequest,
			errorBody{Error: invalid.Reason, Field: invalid.Field, OriginalResponse: origResp})

	case errors.Is(err, common.ErrInvalidInput):
		c.AbortWithStatusJSON(http.StatusBadRequest, errorBody{Error: err.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrNotFound):
		c.AbortWithStatusJSON(http.StatusNotFound, errorBody{Error: err.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrConflict):
		c.AbortWithStatusJSON(http.StatusConflict, errorBody{Error: err.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrUnsupported):
		c.AbortWithStatusJSON(http.StatusUnprocessableEntity, errorBody{Error: err.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrNotImplementedInFafnir):
		c.AbortWithStatusJSON(http.StatusNotImplemented, errorBody{Error: common.ErrNotImplementedInFafnir.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrNotImplementedInIdentityHub):
		c.AbortWithStatusJSON(http.StatusNotImplemented, errorBody{Error: common.ErrNotImplementedInIdentityHub.Error(), OriginalResponse: origResp})

	case errors.Is(err, common.ErrNotLinked):
		c.AbortWithStatusJSON(http.StatusPreconditionFailed, errorBody{Error: err.Error(), OriginalResponse: origResp})

	default:
		// Infrastructure failures return the underlying error description
		// so callers can diagnose issues without having to dig into server logs.
		errMsg := "internal error"
		if err != nil {
			errMsg = err.Error()
		}
		c.AbortWithStatusJSON(http.StatusInternalServerError,
			errorBody{Error: errMsg, OriginalResponse: origResp})
	}
}

// jsonKind names a Go type in the vocabulary of the format the caller wrote in.
// Telling someone their field must be a "[]string" answers in a language they
// were never speaking.
func jsonKind(t reflect.Type) string {
	if t == nil {
		return "of a different type"
	}

	switch t.Kind() {
	case reflect.Slice, reflect.Array:
		return "an array"
	case reflect.Map, reflect.Struct:
		return "an object"
	case reflect.String:
		return "a string"
	case reflect.Bool:
		return "a boolean"
	case reflect.Float32, reflect.Float64,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "a number"
	default:
		return "of a different type"
	}
}
