package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
)

func NewMetrics(ctx context.Context) (*sdkmetric.MeterProvider, error) {
	protocol := os.Getenv("OTEL_EXPORTER_OTLP_METRICS_PROTOCOL")
	if protocol == "" {
		protocol = os.Getenv("OTEL_EXPORTER_OTLP_PROTOCOL")
	}

	var exporter sdkmetric.Exporter
	var err error
	switch protocol {
	case "", "grpc":
		exporter, err = otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithTimeout(2*time.Second))
	case "http/protobuf":
		exporter, err = otlpmetrichttp.New(ctx, otlpmetrichttp.WithTimeout(2*time.Second))
	default:
		return nil, fmt.Errorf("unsupported OTLP metrics protocol %q: use grpc or http/protobuf", protocol)
	}
	if err != nil {
		return nil, fmt.Errorf("initialize OTLP metric exporter: %w", err)
	}

	return sdkmetric.NewMeterProvider(
		sdkmetric.WithResource(serviceResource()),
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(exporter)),
	), nil
}

type TripMetrics struct {
	Created   metric.Int64Counter
	Completed metric.Int64Counter
}

func NewTripMetrics(provider metric.MeterProvider) (TripMetrics, error) {
	meter := provider.Meter("tripgo/trip")
	created, err := meter.Int64Counter("tripgo.trips.created", metric.WithUnit("{trip}"))
	if err != nil {
		return TripMetrics{}, fmt.Errorf("create trips created counter: %w", err)
	}
	completed, err := meter.Int64Counter("tripgo.trips.completed", metric.WithUnit("{trip}"))
	if err != nil {
		return TripMetrics{}, fmt.Errorf("create trips completed counter: %w", err)
	}
	return TripMetrics{Created: created, Completed: completed}, nil
}

func ObserveActiveTrips(provider metric.MeterProvider, count func(context.Context) (int64, error), logger *slog.Logger) error {
	meter := provider.Meter("tripgo/trip")
	active, err := meter.Int64ObservableGauge("tripgo.trips.active", metric.WithUnit("{trip}"))
	if err != nil {
		return fmt.Errorf("create active trips gauge: %w", err)
	}
	_, err = meter.RegisterCallback(func(ctx context.Context, observer metric.Observer) error {
		value, err := count(ctx)
		if err != nil {
			logger.ErrorContext(ctx, "collect active trips failed", "error", err)
			return nil
		}
		observer.ObserveInt64(active, value)
		return nil
	}, active)
	if err != nil {
		return fmt.Errorf("register active trips callback: %w", err)
	}
	return nil
}
