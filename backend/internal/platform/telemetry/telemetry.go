// Package telemetry configures OpenTelemetry tracing.
package telemetry

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"

	"github.com/yelnurq/startify/backend/internal/platform/buildinfo"
	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// Setup installs the global tracer provider. Without an endpoint tracing stays
// a no-op, so instrumentation costs nothing in development.
func Setup(ctx context.Context, cfg config.OTel, service string) (shutdown func(context.Context) error, err error) {
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	if cfg.Endpoint == "" {
		return func(context.Context) error { return nil }, nil
	}

	opts := []otlptracehttp.Option{otlptracehttp.WithEndpointURL(cfg.Endpoint)}
	if cfg.Insecure {
		opts = append(opts, otlptracehttp.WithInsecure())
	}
	exporter, err := otlptracehttp.New(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("telemetry: exporter: %w", err)
	}
	res, err := resource.Merge(resource.Default(), resource.NewWithAttributes(semconv.SchemaURL,
		semconv.ServiceName(service),
		semconv.ServiceVersion(buildinfo.Version),
	))
	if err != nil {
		return nil, fmt.Errorf("telemetry: resource: %w", err)
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// RouteSpanName renames the server span after routing so traces group by
// route template ("GET /api/v1/projects/{id}") instead of raw paths.
func RouteSpanName(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
			trace.SpanFromContext(r.Context()).SetName(r.Method + " " + rc.RoutePattern())
		}
	})
}
