// Package telemetry provides OpenTelemetry HTTP middleware, W3C traceparent extraction, and semantic access logs.
// Uses internal/clock.Stopwatch for duration metrics without mutating proxied headers (DP-1, DP-8, REQ-SYS-003).
package telemetry

import (
	"context"
	"log/slog"
	"net"
	"net/http"
	"strings"

	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
)

// fallbackTP generates W3C trace and span IDs when no explicit Tracer is supplied.
var fallbackTP = sdktrace.NewTracerProvider(sdktrace.WithSampler(sdktrace.AlwaysSample()))

// defaultPropagator extracts and injects W3C TraceContext and Baggage headers.
var defaultPropagator = propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})

// sanitizer strips ASCII line breaks from untrusted request attributes to prevent log forging.
var sanitizer = strings.NewReplacer("\n", "", "\r", "")

// HTTPOptions configures HTTP request tracing and structured access logging.
type HTTPOptions struct {
	Logger     *slog.Logger
	Tracer     trace.Tracer
	Propagator propagation.TextMapPropagator
	Component  string
	Listener   string
}

// HTTPMiddleware wraps next with W3C TraceContext extraction, server span creation, and OTel HTTP access logs.
func HTTPMiddleware(opts HTTPOptions, next http.Handler) http.Handler {
	logger := logx.Default(opts.Logger)
	if opts.Component != "" {
		logger = logx.Component(logger, opts.Component)
	}
	prop := opts.Propagator
	if prop == nil {
		prop = defaultPropagator
	}
	tracer := opts.Tracer
	if tracer == nil {
		tracer = fallbackTP.Tracer("alexandria.http")
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sw := clock.Start()
		method := clean(r.Method)
		urlPath := clean(r.URL.Path)
		ctx := prop.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		ctx, span := tracer.Start(ctx, method+" "+urlPath, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r.WithContext(ctx))

		if rec.status >= http.StatusInternalServerError {
			span.SetStatus(codes.Error, http.StatusText(rec.status))
		}
		lvl := accessLevel(urlPath, rec.status)
		if !logger.Enabled(ctx, lvl) {
			return
		}
		attrs := []slog.Attr{
			slog.String("http.request.method", method),
			slog.String("url.path", urlPath),
			slog.Int("http.response.status_code", rec.status),
			slog.Int64("http.response.body.size", rec.bytes),
			slog.Float64("duration_ms", sw.Seconds()*1000),
		}
		if host := clientAddress(r.RemoteAddr); host != "" {
			attrs = append(attrs, slog.String("client.address", host))
		}
		if opts.Listener != "" {
			attrs = append(attrs, slog.String("listener", opts.Listener))
		}
		logger.LogAttrs(ctx, lvl, "http request completed", attrs...)
	})
}

// ExtractServerSpan extracts W3C traceparent read-only from hdr and starts a server span without mutating hdr.
func ExtractServerSpan(ctx context.Context, hdr http.Header, name string, tracer trace.Tracer) (context.Context, trace.Span) {
	if tracer == nil {
		tracer = fallbackTP.Tracer("alexandria.http")
	}
	ctx = defaultPropagator.Extract(ctx, propagation.HeaderCarrier(hdr))
	return tracer.Start(ctx, clean(name), trace.WithSpanKind(trace.SpanKindServer))
}

// InjectHeaders writes active W3C TraceContext headers from ctx into hdr for outbound infrastructure calls.
func InjectHeaders(ctx context.Context, hdr http.Header) {
	defaultPropagator.Inject(ctx, propagation.HeaderCarrier(hdr))
}

// Clean sanitizes untrusted wire strings before recording them as log attributes.
func Clean(s string) string { return clean(s) }

// clean strips CR and LF characters from s.
func clean(s string) string { return sanitizer.Replace(s) }

// accessLevel selects DEBUG for healthy liveness/readiness probes, WARN for 4xx, ERROR for 5xx, and INFO otherwise.
func accessLevel(urlPath string, status int) slog.Level {
	switch {
	case status >= http.StatusInternalServerError:
		return slog.LevelError
	case status >= http.StatusBadRequest:
		return slog.LevelWarn
	case urlPath == "/healthz" || urlPath == "/readyz":
		return slog.LevelDebug
	default:
		return slog.LevelInfo
	}
}

// clientAddress extracts the host portion of an HTTP RemoteAddr.
func clientAddress(remoteAddr string) string {
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return clean(remoteAddr)
	}
	return clean(host)
}

// statusRecorder captures the HTTP status code and response byte count while preserving ResponseController unwrapping.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int64
	wroteHeader bool
}

// WriteHeader records the first status code written to the response.
func (r *statusRecorder) WriteHeader(code int) {
	if !r.wroteHeader {
		r.status = code
		r.wroteHeader = true
	}
	r.ResponseWriter.WriteHeader(code)
}

// Write counts response payload bytes and marks headers as written.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wroteHeader {
		r.wroteHeader = true
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

// Unwrap exposes the underlying ResponseWriter to net/http.ResponseController.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
