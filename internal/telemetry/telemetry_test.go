// Unit tests for OpenTelemetry SDK setup, JSON/text handlers, W3C trace correlation, and OTLP/HTTP export.
// Verifies Log Data Model fields, simulated clock timestamps, and middleware access logs (DP-1, DP-8, DP-11).
package telemetry_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/ging/alexandria/internal/core/timex"
	"github.com/ging/alexandria/internal/telemetry"
)

// fixedClock returns a deterministic instant for timestamp verification.
type fixedClock struct{ at timex.Instant }

// Now returns the fixed instant.
func (f fixedClock) Now() timex.Instant { return f.at }

// memExporter captures OpenTelemetry SDK log records exported in memory.
type memExporter struct {
	mu      sync.Mutex
	records []sdklog.Record
}

// Export appends the batch of records in memory.
func (m *memExporter) Export(_ context.Context, records []sdklog.Record) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, r := range records {
		m.records = append(m.records, r.Clone())
	}
	return nil
}

// Shutdown satisfies sdklog.Exporter.
func (m *memExporter) Shutdown(context.Context) error { return nil }

// ForceFlush satisfies sdklog.Exporter.
func (m *memExporter) ForceFlush(context.Context) error { return nil }

// count returns the number of captured OTel records.
func (m *memExporter) count() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.records)
}

// parseLines decodes newline-delimited JSON objects from buf.
func parseLines(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(buf.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("unmarshal %q: %v", line, err)
		}
		out = append(out, m)
	}
	return out
}

// mustInstant parses an RFC 3339 string or fails the test.
func mustInstant(t *testing.T, raw string) timex.Instant {
	t.Helper()
	inst, err := timex.ParseInstant(raw)
	if err != nil {
		t.Fatal(err)
	}
	return inst
}

// TestJSONHandlerEmitsOTelLogDataModel verifies timestamp, severity_text, severity_number, body, and resource fields.
func TestJSONHandlerEmitsOTelLogDataModel(t *testing.T) {
	var buf bytes.Buffer
	clk := fixedClock{at: mustInstant(t, "2026-10-10T12:00:00Z")}
	rt, err := telemetry.Setup(t.Context(), telemetry.Config{
		ServiceName:    "alexandria-server",
		ServiceVersion: "1.2.3",
		Environment:    "lab",
		Level:          "info",
		Format:         "json",
		Output:         &buf,
		Clock:          clk,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown(t.Context()) }()

	rt.Logger.Debug("filtered out")
	rt.Logger.Warn("disk threshold", slog.Int("used_pct", 88))

	lines := parseLines(t, &buf)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := lines[0]
	if got["timestamp"] != "2026-10-10T12:00:00Z" || got["severity_text"] != "WARN" || got["severity_number"] != float64(13) ||
		got["body"] != "disk threshold" || got["service.name"] != "alexandria-server" || got["service.version"] != "1.2.3" ||
		got["deployment.environment.name"] != "lab" || got["used_pct"] != float64(88) {
		t.Fatalf("unexpected OTel JSON record: %#v", got)
	}
}

// TestW3CTraceparentCorrelation verifies that incoming W3C traceparent headers propagate trace_id and span_id to logs.
func TestW3CTraceparentCorrelation(t *testing.T) {
	var buf bytes.Buffer
	exp := &memExporter{}
	rt, err := telemetry.Setup(t.Context(), telemetry.Config{
		ServiceName: "alexandria-server",
		Level:       "info",
		Format:      "json",
		Output:      &buf,
		Processors:  []sdklog.Processor{sdklog.NewSimpleProcessor(exp)},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown(t.Context()) }()

	wantTraceID := "4bf92f3577b34da6a3ce929d0e0e4736"
	h := telemetry.HTTPMiddleware(telemetry.HTTPOptions{
		Logger:     rt.Logger,
		Tracer:     rt.Tracer,
		Propagator: rt.Propagator,
		Component:  "http",
	}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rt.Logger.InfoContext(r.Context(), "handling namespace")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/v1/namespaces", nil)
	req.Header.Set("traceparent", "00-"+wantTraceID+"-00f067aa0ba902b7-01")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)

	lines := parseLines(t, &buf)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	for i, line := range lines {
		if line["trace_id"] != wantTraceID || line["span_id"] == "" {
			t.Fatalf("line %d missing trace correlation: %#v", i, line)
		}
	}
	if exp.count() != 2 {
		t.Fatalf("got %d OTel SDK records, want 2", exp.count())
	}
}

// TestOTLPHTTPExportPostsProtobufBatch verifies active OTLP/HTTP export to a collector endpoint on Shutdown.
func TestOTLPHTTPExportPostsProtobufBatch(t *testing.T) {
	var mu sync.Mutex
	var receivedPaths []string
	var receivedBytes int
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		receivedPaths = append(receivedPaths, r.URL.Path)
		receivedBytes += len(b)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/x-protobuf")
		w.WriteHeader(http.StatusOK)
	}))
	defer collector.Close()

	var buf bytes.Buffer
	rt, err := telemetry.Setup(t.Context(), telemetry.Config{
		ServiceName:  "alexandria-client",
		Level:        "info",
		Format:       "text",
		OTLPEndpoint: collector.URL,
		Output:       &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	rt.Logger.Info("sync completed", slog.Uint64("sequence", 4))
	if err := rt.Shutdown(t.Context()); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(receivedPaths) == 0 || receivedPaths[0] != "/v1/logs" || receivedBytes == 0 {
		t.Fatalf("collector received paths=%v bytes=%d", receivedPaths, receivedBytes)
	}
	if !strings.Contains(buf.String(), "sync completed") {
		t.Fatalf("expected text console output, got %q", buf.String())
	}
}

// TestHealthProbeDemotedToDebug verifies that 200 /healthz logs at DEBUG while 5xx logs at ERROR.
func TestHealthProbeDemotedToDebug(t *testing.T) {
	var buf bytes.Buffer
	rt, err := telemetry.Setup(t.Context(), telemetry.Config{
		ServiceName: "alexandria-client",
		Level:       "info",
		Format:      "json",
		Output:      &buf,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = rt.Shutdown(t.Context()) }()

	h := telemetry.HTTPMiddleware(telemetry.HTTPOptions{Logger: rt.Logger, Listener: "admin"}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/readyz" {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/healthz", nil))
	if buf.Len() != 0 {
		t.Fatalf("expected /healthz 200 to be suppressed at info level, got %s", buf.String())
	}
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/readyz", nil))
	lines := parseLines(t, &buf)
	if len(lines) != 1 || lines[0]["severity_text"] != "ERROR" || lines[0]["listener"] != "admin" {
		t.Fatalf("unexpected /readyz 503 log: %#v", lines)
	}
}

// TestParseLevelAndValidateFormatRejectInvalidValues verifies configuration validation errors.
func TestParseLevelAndValidateFormatRejectInvalidValues(t *testing.T) {
	if _, err := telemetry.ParseLevel("verbose"); err == nil {
		t.Fatal("expected error on invalid level")
	}
	if err := telemetry.ValidateFormat("xml"); err == nil {
		t.Fatal("expected error on invalid format")
	}
}
