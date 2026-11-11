// Package proxy_dsp provides the L7 proxy for dsp-ingress, mgmt-ingress and dsp-egress: reads the body as bytes, calls Edge and writes its response.
// Uses a raw RoundTrip instead of httputil.ReverseProxy so no header (X-Forwarded-For, User-Agent, Accept-Encoding) is added (REQ-SYS-003, REQ-CLI-014, REQ-CLI-019, REQ-CLI-022).
package proxy_dsp

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/client/app"
	"github.com/ging/alexandria/internal/client/domain"
	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
	"github.com/ging/alexandria/internal/telemetry"
)

// LatencyObserver receives the wall-clock latency of each request.
type LatencyObserver interface{ ObserveLatency(seconds float64) }

// Handler serves one traffic listener.
type Handler struct {
	Edge     *app.Edge
	Listener domain.Listener
	Latency  LatencyObserver
	Logger   *slog.Logger
	Tracer   trace.Tracer
}

// ServeHTTP proxies one request.
func (h Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	sw := clock.Start()
	ctx, span := telemetry.ExtractServerSpan(r.Context(), r.Header, r.Method+" "+r.URL.Path, h.Tracer)
	defer span.End()

	body, err := io.ReadAll(io.LimitReader(r.Body, 32<<20))
	if err != nil {
		logx.Default(h.Logger).WarnContext(ctx, "failed to read dsp request body", slog.String("listener", string(h.Listener)), logx.Err(err))
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req := &app.Request{Listener: h.Listener, Method: r.Method, Path: r.URL.Path, RawQuery: r.URL.RawQuery, Host: r.Host, Header: r.Header.Clone(), Body: body}
	if h.Listener == domain.DSPEgress && r.URL.IsAbs() {
		u := *r.URL
		req.Target = &u
	}
	resp := h.Edge.Handle(req)
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
	if resp.Status >= http.StatusInternalServerError {
		lvl = slog.LevelError
	} else if resp.Status >= http.StatusBadRequest {
		lvl = slog.LevelWarn
	}
	logx.Default(h.Logger).LogAttrs(ctx, lvl, "dsp request processed",
		slog.String("listener", string(h.Listener)),
		slog.String("http.request.method", telemetry.Clean(r.Method)),
		slog.String("url.path", telemetry.Clean(r.URL.Path)),
		slog.Int("http.response.status_code", resp.Status),
		slog.Int("http.response.body.size", len(resp.Body)),
		slog.Float64("duration_ms", elapsed*1000),
	)
}

// Upstream forwards requests byte for byte to the connector or the egress destination.
type Upstream struct {
	DSP        *url.URL
	Management *url.URL
	Transport  *http.Transport
}

var _ app.Upstream = (*Upstream)(nil)

// NewUpstream builds an upstream with compression and proxies disabled.
func NewUpstream(dsp, mgmt string) (*Upstream, error) {
	u := &Upstream{Transport: &http.Transport{DisableCompression: true, Proxy: nil, ResponseHeaderTimeout: 60 * time.Second}}
	var err error
	if u.DSP, err = url.Parse(dsp); err != nil {
		return nil, err
	}
	if mgmt != "" {
		if u.Management, err = url.Parse(mgmt); err != nil {
			return nil, err
		}
	}
	return u, nil
}

// target computes the destination URL of a request.
func (u *Upstream) target(r *app.Request) (*url.URL, error) {
	if r.Listener == domain.DSPEgress {
		if r.Target == nil {
			return nil, fmt.Errorf("proxy_dsp: dsp-egress needs an absolute request URI")
		}
		return r.Target, nil
	}
	base := u.DSP
	if r.Listener == domain.MgmtIngress {
		base = u.Management
	}
	if base == nil {
		return nil, fmt.Errorf("proxy_dsp: no upstream for %s", r.Listener)
	}
	t := *base
	t.Path = path.Join(base.Path, r.Path)
	if strings.HasSuffix(r.Path, "/") && !strings.HasSuffix(t.Path, "/") {
		t.Path += "/"
	}
	t.RawQuery = r.RawQuery
	return &t, nil
}

// Forward sends the request with its original headers and body and returns the raw response.
func (u *Upstream) Forward(r *app.Request) (*app.Response, error) {
	t, err := u.target(r)
	if err != nil {
		return nil, err
	}
	out, err := http.NewRequestWithContext(context.Background(), r.Method, t.String(), bytes.NewReader(r.Body))
	if err != nil {
		return nil, err
	}
	out.Header = r.Header.Clone()
	if _, ok := out.Header["User-Agent"]; !ok {
		// An explicitly empty User-Agent stops net/http from adding its own.
		out.Header["User-Agent"] = []string{""}
	}
	out.Host = r.Host
	resp, err := u.Transport.RoundTrip(out)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return &app.Response{Status: resp.StatusCode, Header: resp.Header.Clone(), Body: b}, nil
}
