// Package telemetry configures OpenTelemetry SDK logging, W3C trace propagation, and structured slog handlers.
// Bridges slog records to both console streams (JSON or text) and OTLP/HTTP log exporters (DP-1, DP-8, DP-11).
package telemetry

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	sdklog "go.opentelemetry.io/otel/sdk/log"

	"github.com/ging/alexandria/internal/core/timex"
)

// DefaultVersion is the fallback service.version reported when none is configured.
const DefaultVersion = "0.1.0-poc"

// Config defines the logging, resource, and OTLP export settings of a process.
type Config struct {
	ServiceName    string
	ServiceVersion string
	Environment    string
	Level          string
	Format         string
	OTLPEndpoint   string
	OTLPHeaders    map[string]string
	OTLPInsecure   bool
	Output         io.Writer
	Clock          timex.Clock
	AddSource      bool
	SetGlobal      bool
	Processors     []sdklog.Processor
}

// normalized returns a copy of Config with environment overrides and defaults applied.
func (c Config) normalized() Config {
	if env := strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME")); env != "" {
		c.ServiceName = env
	}
	if c.ServiceName == "" {
		c.ServiceName = "alexandria"
	}
	if env := strings.TrimSpace(os.Getenv("OTEL_SERVICE_VERSION")); env != "" {
		c.ServiceVersion = env
	}
	if c.ServiceVersion == "" {
		c.ServiceVersion = DefaultVersion
	}
	if env := firstEnv("OTEL_DEPLOYMENT_ENVIRONMENT_NAME", "OTEL_DEPLOYMENT_ENVIRONMENT"); env != "" {
		c.Environment = env
	}
	if env := firstEnv("ALEXANDRIA_LOG_LEVEL", "OTEL_LOG_LEVEL"); env != "" {
		c.Level = env
	}
	if c.Level == "" {
		c.Level = "info"
	}
	if env := strings.TrimSpace(os.Getenv("ALEXANDRIA_LOG_FORMAT")); env != "" {
		c.Format = env
	}
	if c.Format == "" {
		c.Format = "auto"
	}
	if c.OTLPEndpoint == "" {
		c.OTLPEndpoint = firstEnv("OTEL_EXPORTER_OTLP_LOGS_ENDPOINT", "OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if strings.EqualFold(firstEnv("OTEL_EXPORTER_OTLP_LOGS_INSECURE", "OTEL_EXPORTER_OTLP_INSECURE"), "true") {
		c.OTLPInsecure = true
	}
	if c.Output == nil {
		c.Output = os.Stderr
	}
	return c
}

// firstEnv returns the first non-empty trimmed environment variable from keys.
func firstEnv(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

// ParseLevel converts a level name (debug, info, warn, error, none) into slog.Level.
func ParseLevel(raw string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "info":
		return slog.LevelInfo, nil
	case "debug":
		return slog.LevelDebug, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	case "none", "off", "disabled":
		return slog.LevelError + 8, nil
	default:
		return slog.LevelInfo, fmt.Errorf("unknown log level %q (expected debug, info, warn, error, or none)", raw)
	}
}

// ValidateFormat checks whether raw is a supported log output format (auto, json, text).
func ValidateFormat(raw string) error {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "auto", "json", "text":
		return nil
	default:
		return fmt.Errorf("unknown log format %q (expected auto, json, or text)", raw)
	}
}

// ResolveFormat resolves "auto" to "text" on interactive character terminals and "json" everywhere else.
func ResolveFormat(raw string, out io.Writer) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "json":
		return "json"
	case "text":
		return "text"
	default:
		if isTerminal(out) {
			return "text"
		}
		return "json"
	}
}

// isTerminal reports whether w is an os.File backed by a character device.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok || f == nil {
		return false
	}
	st, err := f.Stat()
	if err != nil {
		return false
	}
	return st.Mode()&os.ModeCharDevice != 0
}
