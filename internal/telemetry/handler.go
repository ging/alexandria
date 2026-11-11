// Package telemetry implements OpenTelemetry Log Data Model slog handlers, trace correlation, and multi-sink fanout.
// Emits RFC 3339 timestamps, severity_text, severity_number, body, trace_id, and span_id (DP-1, DP-8, DP-11).
package telemetry

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/timex"
)

// SeverityNumber maps an slog.Level to its OpenTelemetry Logs Data Model SeverityNumber (1..24).
func SeverityNumber(l slog.Level) int {
	switch {
	case l < slog.LevelInfo:
		return max(1, 9+int(l))
	case l < slog.LevelWarn:
		return 9 + int(l-slog.LevelInfo)
	case l < slog.LevelError:
		return 13 + int(l-slog.LevelWarn)
	default:
		return min(24, 17+int(l-slog.LevelError))
	}
}

// SeverityText maps an slog.Level to its OpenTelemetry canonical severity label.
func SeverityText(l slog.Level) string {
	switch {
	case l < slog.LevelInfo:
		return "DEBUG"
	case l < slog.LevelWarn:
		return "INFO"
	case l < slog.LevelError:
		return "WARN"
	default:
		return "ERROR"
	}
}

// newStreamHandler creates a JSON or text slog.Handler enriched with OpenTelemetry fields and trace context.
func newStreamHandler(out io.Writer, format string, level slog.Leveler, addSource bool, clk timex.Clock, baseAttrs []slog.Attr) slog.Handler {
	jsonMode := format == "json"
	opts := &slog.HandlerOptions{
		Level:     level,
		AddSource: addSource,
	}
	var inner slog.Handler
	if jsonMode {
		opts.ReplaceAttr = replaceOTelJSONAttr
		inner = slog.NewJSONHandler(out, opts)
	} else {
		inner = slog.NewTextHandler(out, opts)
	}
	if len(baseAttrs) > 0 {
		inner = inner.WithAttrs(baseAttrs)
	}
	_, isSys := clk.(clock.System)
	return &enrichHandler{
		next:         inner,
		clock:        clk,
		overrideTime: clk != nil && !isSys,
		jsonMode:     jsonMode,
	}
}

// replaceOTelJSONAttr rewrites top-level slog built-in keys to OpenTelemetry Log Data Model names.
func replaceOTelJSONAttr(groups []string, a slog.Attr) slog.Attr {
	if len(groups) > 0 {
		return a
	}
	switch a.Key {
	case slog.TimeKey:
		return slog.String("timestamp", a.Value.Time().UTC().Format(time.RFC3339Nano))
	case slog.LevelKey:
		if l, ok := a.Value.Any().(slog.Level); ok {
			return slog.String("severity_text", SeverityText(l))
		}
	case slog.MessageKey:
		return slog.String("body", a.Value.String())
	case slog.SourceKey:
		if src, ok := a.Value.Any().(*slog.Source); ok && src != nil {
			return slog.Group("code",
				slog.String("filepath", src.File),
				slog.String("function", src.Function),
				slog.Int("lineno", src.Line),
			)
		}
	}
	return a
}

// enrichHandler injects simulated clock instants, severity_number, and active W3C trace/span identifiers.
type enrichHandler struct {
	next         slog.Handler
	clock        timex.Clock
	overrideTime bool
	jsonMode     bool
}

// Enabled delegates level filtering to the wrapped handler.
func (h *enrichHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

// Handle enriches a cloned record with OpenTelemetry metadata before delegating to next.
func (h *enrichHandler) Handle(ctx context.Context, rec slog.Record) error {
	r := rec.Clone()
	if h.overrideTime {
		r.Time = h.clock.Now().Time().UTC()
	} else if r.Time.IsZero() {
		r.Time = clock.System{}.Now().Time().UTC()
	}
	if h.jsonMode {
		r.AddAttrs(slog.Int("severity_number", SeverityNumber(r.Level)))
	}
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
		if h.jsonMode {
			r.AddAttrs(slog.String("trace_flags", sc.TraceFlags().String()))
		}
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs returns a new enrichHandler with additional pre-formatted attributes.
func (h *enrichHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &enrichHandler{
		next:         h.next.WithAttrs(attrs),
		clock:        h.clock,
		overrideTime: h.overrideTime,
		jsonMode:     h.jsonMode,
	}
}

// WithGroup returns a new enrichHandler scoped to name.
func (h *enrichHandler) WithGroup(name string) slog.Handler {
	return &enrichHandler{
		next:         h.next.WithGroup(name),
		clock:        h.clock,
		overrideTime: h.overrideTime,
		jsonMode:     h.jsonMode,
	}
}

// levelFilterHandler gates an slog.Handler (such as otelslog) behind a dynamic slog.Leveler.
type levelFilterHandler struct {
	level        slog.Leveler
	next         slog.Handler
	clock        timex.Clock
	overrideTime bool
}

// Enabled reports whether level meets the configured threshold and next is enabled.
func (h *levelFilterHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.level.Level() && h.next.Enabled(ctx, level)
}

// Handle forwards the record to next, applying simulated clock overrides when configured.
func (h *levelFilterHandler) Handle(ctx context.Context, rec slog.Record) error {
	r := rec.Clone()
	if h.overrideTime {
		r.Time = h.clock.Now().Time().UTC()
	}
	return h.next.Handle(ctx, r)
}

// WithAttrs returns a level-filtered handler with additional attributes.
func (h *levelFilterHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &levelFilterHandler{level: h.level, next: h.next.WithAttrs(attrs), clock: h.clock, overrideTime: h.overrideTime}
}

// WithGroup returns a level-filtered handler scoped to name.
func (h *levelFilterHandler) WithGroup(name string) slog.Handler {
	return &levelFilterHandler{level: h.level, next: h.next.WithGroup(name), clock: h.clock, overrideTime: h.overrideTime}
}

// fanoutHandler broadcasts every log record to multiple slog.Handler sinks.
type fanoutHandler struct {
	handlers []slog.Handler
}

// Enabled reports true if at least one sink accepts level.
func (f *fanoutHandler) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range f.handlers {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

// Handle dispatches a cloned record to each enabled sink and joins any errors.
func (f *fanoutHandler) Handle(ctx context.Context, rec slog.Record) error {
	var errs []error
	for _, h := range f.handlers {
		if h.Enabled(ctx, rec.Level) {
			if err := h.Handle(ctx, rec.Clone()); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

// WithAttrs returns a fanoutHandler with attrs applied to every sink.
func (f *fanoutHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithAttrs(attrs)
	}
	return &fanoutHandler{handlers: out}
}

// WithGroup returns a fanoutHandler with name applied to every sink.
func (f *fanoutHandler) WithGroup(name string) slog.Handler {
	out := make([]slog.Handler, len(f.handlers))
	for i, h := range f.handlers {
		out[i] = h.WithGroup(name)
	}
	return &fanoutHandler{handlers: out}
}
