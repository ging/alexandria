// Package telemetry manages the OpenTelemetry Runtime lifecycle: Resource, LoggerProvider, TracerProvider, and slog fanout.
// Supports optional OTLP/HTTP active export alongside structured console logging (DP-1, DP-8, DP-11).
package telemetry

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/url"
	"strings"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp"
	"go.opentelemetry.io/otel/propagation"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"

	"github.com/ging/alexandria/internal/clock"
	"github.com/ging/alexandria/internal/core/logx"
)

// Runtime holds the initialized logger, OpenTelemetry providers, W3C propagator, and level controller.
type Runtime struct {
	Logger         *slog.Logger
	Level          *slog.LevelVar
	Tracer         trace.Tracer
	Propagator     propagation.TextMapPropagator
	LoggerProvider *sdklog.LoggerProvider
	TracerProvider *sdktrace.TracerProvider
}

// Setup initializes structured console logging and OpenTelemetry SDK pipelines from cfg.
func Setup(ctx context.Context, cfg Config) (*Runtime, error) {
	c := cfg.normalized()
	lvl, err := ParseLevel(c.Level)
	if err != nil {
		return nil, err
	}
	if err := ValidateFormat(c.Format); err != nil {
		return nil, err
	}
	levelVar := &slog.LevelVar{}
	levelVar.Set(lvl)

	res, err := buildResource(ctx, c)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
	)
	prop := propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})
	tracer := tp.Tracer(c.ServiceName)

	baseAttrs := []slog.Attr{
		slog.String("service.name", c.ServiceName),
		slog.String("service.version", c.ServiceVersion),
	}
	if c.Environment != "" {
		baseAttrs = append(baseAttrs, slog.String("deployment.environment.name", c.Environment))
	}
	addSource := c.AddSource || lvl <= slog.LevelDebug
	streamFmt := ResolveFormat(c.Format, c.Output)
	stream := newStreamHandler(c.Output, streamFmt, levelVar, addSource, c.Clock, baseAttrs)

	processors := append([]sdklog.Processor(nil), c.Processors...)
	if c.OTLPEndpoint != "" {
		exp, err := newOTLPExporter(ctx, c)
		if err != nil {
			_ = tp.Shutdown(ctx)
			return nil, err
		}
		processors = append(processors, sdklog.NewBatchProcessor(exp))
	}

	handlers := []slog.Handler{stream}
	var lp *sdklog.LoggerProvider
	if len(processors) > 0 {
		lpOpts := []sdklog.LoggerProviderOption{sdklog.WithResource(res)}
		for _, p := range processors {
			lpOpts = append(lpOpts, sdklog.WithProcessor(p))
		}
		lp = sdklog.NewLoggerProvider(lpOpts...)
		_, isSys := c.Clock.(clock.System)
		otelBridge := otelslog.NewHandler(
			c.ServiceName,
			otelslog.WithLoggerProvider(lp),
			otelslog.WithVersion(c.ServiceVersion),
			otelslog.WithSource(addSource),
		)
		handlers = append(handlers, &levelFilterHandler{
			level:        levelVar,
			next:         otelBridge,
			clock:        c.Clock,
			overrideTime: c.Clock != nil && !isSys,
		})
	}

	root := stream
	if len(handlers) > 1 {
		root = &fanoutHandler{handlers: handlers}
	}
	logger := slog.New(root)

	if c.SetGlobal {
		slog.SetDefault(logger)
		otel.SetTracerProvider(tp)
		otel.SetTextMapPropagator(prop)
		if lp != nil {
			otel.SetLoggerProvider(lp)
		}
		otel.SetErrorHandler(otel.ErrorHandlerFunc(func(oerr error) {
			logger.Warn("opentelemetry internal error", logx.Err(oerr))
		}))
	}

	return &Runtime{
		Logger:         logger,
		Level:          levelVar,
		Tracer:         tracer,
		Propagator:     prop,
		LoggerProvider: lp,
		TracerProvider: tp,
	}, nil
}

// ErrorLog returns a standard library *log.Logger bridged into the structured logger at WARN level.
func (r *Runtime) ErrorLog() *log.Logger {
	if r == nil || r.Logger == nil {
		return slog.NewLogLogger(logx.Nop().Handler(), slog.LevelWarn)
	}
	return slog.NewLogLogger(r.Logger.Handler(), slog.LevelWarn)
}

// Shutdown flushes and closes the OpenTelemetry log and trace providers cleanly.
func (r *Runtime) Shutdown(ctx context.Context) error {
	if r == nil {
		return nil
	}
	var errs []error
	if r.LoggerProvider != nil {
		if err := r.LoggerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	if r.TracerProvider != nil {
		if err := r.TracerProvider.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// buildResource constructs the OpenTelemetry Resource describing the running service instance.
func buildResource(ctx context.Context, c Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		attribute.String("service.name", c.ServiceName),
		attribute.String("service.version", c.ServiceVersion),
	}
	if c.Environment != "" {
		attrs = append(attrs, attribute.String("deployment.environment.name", c.Environment))
	}
	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithProcessPID(),
		resource.WithHost(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
	if err != nil && res == nil {
		return nil, err
	}
	return res, nil
}

// newOTLPExporter creates an OTLP/HTTP log exporter from normalized configuration.
func newOTLPExporter(ctx context.Context, c Config) (*otlploghttp.Exporter, error) {
	var opts []otlploghttp.Option
	ep := strings.TrimSpace(c.OTLPEndpoint)
	if strings.HasPrefix(ep, "http://") || strings.HasPrefix(ep, "https://") {
		u, err := url.Parse(ep)
		if err != nil {
			return nil, err
		}
		if u.Path == "" || u.Path == "/" {
			u.Path = "/v1/logs"
		}
		opts = append(opts, otlploghttp.WithEndpointURL(u.String()))
	} else if ep != "" {
		opts = append(opts, otlploghttp.WithEndpoint(ep))
		if c.OTLPInsecure {
			opts = append(opts, otlploghttp.WithInsecure())
		}
	}
	if len(c.OTLPHeaders) > 0 {
		opts = append(opts, otlploghttp.WithHeaders(c.OTLPHeaders))
	}
	return otlploghttp.New(ctx, opts...)
}
