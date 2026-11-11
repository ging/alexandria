// Unit tests for nil-safe logger defaults, component scoping, and coded error attribute formatting.
// Verifies inner-layer structured log attributes without external telemetry dependencies (DP-1, DP-11).
package logx_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/ging/alexandria/internal/core/codes"
	"github.com/ging/alexandria/internal/core/logx"
)

// newJSONLogger builds an in-memory JSON logger and returns it alongside its backing buffer.
func newJSONLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewJSONHandler(&buf, nil)), &buf
}

// decodeLine unmarshals a single JSON log line from buf.
func decodeLine(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("unmarshal log: %v", err)
	}
	return m
}

// TestDefaultReturnsNopWhenNil verifies that nil loggers safely fall back to a no-op logger.
func TestDefaultReturnsNopWhenNil(t *testing.T) {
	logx.Default(nil).Info("ignored")
	logx.Component(nil, "hub").Info("ignored")
	if logx.Nop() == nil {
		t.Fatal("expected non-nil nop logger")
	}
}

// TestComponentAddsAttribute verifies that Component scopes records with the component field.
func TestComponentAddsAttribute(t *testing.T) {
	l, buf := newJSONLogger()
	logx.Component(l, "sync").Info("started")
	got := decodeLine(t, buf)
	if got["component"] != "sync" {
		t.Fatalf("got component=%v, want sync", got["component"])
	}
}

// TestErrExtractsCatalogCode verifies that coded errors emit both error.code and error.message.
func TestErrExtractsCatalogCode(t *testing.T) {
	l, buf := newJSONLogger()
	l.Error("failed", logx.Err(codes.New(codes.APINotFound, "namespace demo not found")))
	got := decodeLine(t, buf)
	errGroup, ok := got["error"].(map[string]any)
	if !ok || errGroup["code"] != string(codes.APINotFound) || errGroup["message"] != "namespace demo not found" {
		t.Fatalf("unexpected error group: %#v", got["error"])
	}
}

// TestErrPlainError verifies that uncoded errors emit error.message cleanly.
func TestErrPlainError(t *testing.T) {
	l, buf := newJSONLogger()
	l.Error("failed", logx.Err(errors.New("disk full")), logx.Code(codes.ReleaseAborted))
	got := decodeLine(t, buf)
	errGroup, ok := got["error"].(map[string]any)
	if !ok || errGroup["message"] != "disk full" || got["error.code"] != string(codes.ReleaseAborted) {
		t.Fatalf("unexpected payload: %#v", got)
	}
}
