// Package logx provides nil-safe structured logging helpers and OpenTelemetry semantic attribute builders for inner layers.
// Keeps application use cases decoupled from SDK infrastructure while preserving coded error metadata (DP-1, DP-11).
package logx

import (
	"log/slog"

	"github.com/ging/alexandria/internal/core/codes"
)

// discard is the shared no-op logger returned when no logger is wired.
var discard = slog.New(slog.DiscardHandler)

// Nop returns a logger that discards every record without allocation overhead.
func Nop() *slog.Logger { return discard }

// Default returns l when non-nil, or the shared no-op logger otherwise.
func Default(l *slog.Logger) *slog.Logger {
	if l == nil {
		return discard
	}
	return l
}

// Component returns a child logger tagged with the given architectural component name.
func Component(l *slog.Logger, name string) *slog.Logger {
	if l == nil {
		return discard
	}
	return l.With(slog.String("component", name))
}

// Err builds OpenTelemetry error attributes, including error.code when err carries a catalog code.
func Err(err error) slog.Attr {
	if err == nil {
		return slog.Attr{}
	}
	if ce, ok := codes.As(err); ok {
		return slog.Group("error",
			slog.String("code", string(ce.Code)),
			slog.String("message", ce.Detail),
		)
	}
	return slog.Group("error", slog.String("message", err.Error()))
}

// Code builds an error.code attribute from a catalog code.
func Code(c codes.Code) slog.Attr {
	return slog.String("error.code", string(c))
}
