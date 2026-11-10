package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

var Version = "dev"

func NewTracing(ctx context.Context, logger *slog.Logger) (*sdktrace.TracerProvider, error) {
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		logger.ErrorContext(context.WithoutCancel(ctx), "telemetry export failed", "error", err)
	}))

	protocol := os.Getenv("OTEL_EXPORTER_OTLP_TRACES_PROTOCOL")
	if protocol == "" {
		protocol = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}

	var exporter sdktrace.SpanExporter
	var err error
	switch protocol {
	case "", "grpc":
		exporter, err = otlptracegrpc.New(ctx)
	case "http/protobuf":
		exporter, err = otlptracehttp.New(ctx)
	default:
		return nil, fmt.Errorf("unsupported OTLP traces protocol %q: use grpc or http/protobuf", protocol)
	}
	if err != nil {
		return nil, fmt.Errorf("initialize OTLP trace exporter: %w", err)
	}

	return sdktrace.NewTracerProvider(
		sdktrace.WithResource(serviceResource()),
		sdktrace.WithBatcher(exporter),
	), nil
}

func serviceResource() *resource.Resource {
	return resource.NewSchemaless(
		attribute.String("service.name", "trip-service"),
		attribute.String("service.version", Version),
		attribute.String("service.namespace", "tripgo"),
		attribute.String("deployment.environment", "local"),
	)
}
