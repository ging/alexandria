// Package common declares shared domain error sentinels and error constructor helpers.
// It defines standard validation and conflict errors used across bounded contexts.
package common

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound reports that a referenced entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrConflict reports that the operation collides with existing state.
	ErrConflict = errors.New("conflict")
	// ErrInvalidInput reports that the caller supplied unusable arguments.
	ErrInvalidInput = errors.New("invalid input")
	// ErrNotLinked reports that the wallet is not registered in the directory.
	ErrNotLinked = errors.New("wallet is not linked")
	// ErrUnsupported reports that the request names a capability this build
	// does not implement.
	ErrUnsupported = errors.New("unsupported")
	// ErrNotImplementedInFafnir reports that an operation is unsupported by Fafnir.
	ErrNotImplementedInFafnir = errors.New("Error not implemented in fafnir wallet")
	// ErrNotImplementedInIdentityHub reports that an operation is unsupported by IdentityHub.
	ErrNotImplementedInIdentityHub = errors.New("Error not implemented in IdentityHub wallet")
)

// ValidationError pinpoints the offending field of an invalid request, so a
// driving adapter can render a field-level message without parsing strings.
type ValidationError struct {
	Field  string
	Reason string
}

// Error implements error.
func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Reason)
}

// Is makes every ValidationError match ErrInvalidInput, so adapters can handle
// the whole class with a single errors.Is.
func (e ValidationError) Is(target error) bool { return target == ErrInvalidInput }

// Invalid is the shorthand the use cases build their rejections with, naming the
// offending field and why it was refused.
func Invalid(field, reason string) error {
	return ValidationError{Field: field, Reason: reason}
}

// UpstreamError represents an error returned by an external wallet provider (e.g. IdentityHub or Fafnir).
type UpstreamError struct {
	Provider   string `json:"provider"`
	StatusCode int    `json:"statusCode"`
	Path       string `json:"path,omitempty"`
	RawBody    []byte `json:"body,omitempty"`
	Sentinel   error  `json:"-"`
}

// Error implements error.
func (e *UpstreamError) Error() string {
	if len(e.RawBody) > 0 {
		return fmt.Sprintf("%s: %s returned %d: %s: %v", e.Provider, e.Path, e.StatusCode, string(e.RawBody), e.Sentinel)
	}
	return fmt.Sprintf("%s: %s returned %d: %v", e.Provider, e.Path, e.StatusCode, e.Sentinel)
}

// Unwrap exposes the underlying domain sentinel to errors.Is and errors.As.
func (e *UpstreamError) Unwrap() error {
	return e.Sentinel
}

// Is allows errors.Is(err, sentinel) to match against e.Sentinel.
func (e *UpstreamError) Is(target error) bool {
	return errors.Is(e.Sentinel, target)
}

// NewUpstreamError creates an UpstreamError with a matching domain sentinel.
func NewUpstreamError(provider string, statusCode int, path string, body []byte, sentinel error) *UpstreamError {
	return &UpstreamError{
		Provider:   provider,
		StatusCode: statusCode,
		Path:       path,
		RawBody:    body,
		Sentinel:   sentinel,
	}
}
