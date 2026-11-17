// Package authz_http serves the traffic listeners in validator mode: an external proxy (Envoy ext_authz HTTP service) asks, the client only decides.
// The check carries the original method, path and body; 200 allows and any other status is the §4.8 answer to relay (REQ-CLI-012, REQ-CLI-013, REQ-CLI-019).
package authz_http

import (
	"io"
	"log/slog"
	"net/http"

	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/telemetry"
)

// MaxBody is the largest message a check may carry, the same bound as the L7 proxy.
const MaxBody = 32 << 20

// LatencyObserver receives the wall-clock latency of each check.
type LatencyObserver interface{ ObserveLatency(seconds float64) }

// Handler answers the checks of one traffic listener.
type Handler struct {
	Edge     *app.Edge
	Listener domain.Listener
	Latency  LatencyObserver
	Logger   *slog.Logger
	Tracer   trace.Tracer
}

// ServeHTTP decides on one check request without forwarding it.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sw := clock.Start()
	ctx, span := telemetry.ExtractServerSpan(r.Context(), r.Header, r.Method+" "+r.URL.Path, h.Tracer)
	defer span.End()

	body, err := io.ReadAll(io.LimitReader(r.Body, MaxBody))
	if err != nil {
		logx.Default(h.Logger).WarnContext(ctx, "failed to read check request body", slog.String("listener", string(h.Listener)), logx.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := h.Edge.Check(&app.Request{Listener: h.Listener, Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Host: r.Host, Header: r.Header.Clone(), Body: body})
	for k, vs := range resp.Header {
		if http.CanonicalHeaderKey(k) == http.CanonicalHeaderKey(domain.HeaderName) {
			// Keep the spelling of §4.8 on the wire; header names are case-insensitive anyway.
			w.Header()[domain.HeaderName] = vs
			continue
		}
		w.Header()[k] = vs
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write(resp.Body)
	elapsed := sw.Seconds()
	if h.Latency != nil {
		h.Latency.ObserveLatency(elapsed)
	}
	lvl := slog.LevelInfo
	if resp.Status >= http.StatusBadRequest {
		lvl = slog.LevelWarn
	}
	logx.Default(h.Logger).LogAttrs(ctx, lvl, "dsp check answered",
		slog.String("listener", string(h.Listener)),
		slog.String("http.request.method", telemetry.Clean(r.Method)),
		slog.String("url.path", telemetry.Clean(r.URL.Path)),
		slog.Int("http.response.status_code", resp.Status),
		slog.Bool("allowed", resp.Status == http.StatusOK),
		slog.Float64("duration_ms", elapsed*1000),
	)
}
