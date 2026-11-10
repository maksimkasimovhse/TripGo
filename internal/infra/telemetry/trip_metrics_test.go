package telemetry

import (
	"context"
	"errors"
	"log/slog"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func TestTripMetrics(t *testing.T) {
	t.Parallel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	counters, err := NewTripMetrics(provider)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	counters.Created.Add(ctx, 2)
	counters.Completed.Add(ctx, 1)
	active := int64(8)
	var countErr error
	if err := ObserveActiveTrips(provider, func(context.Context) (int64, error) {
		return active, countErr
	}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	for _, wantActive := range []int64{8, 7} {
		active = wantActive
		var data metricdata.ResourceMetrics
		if err := reader.Collect(ctx, &data); err != nil {
			t.Fatal(err)
		}
		found := make(map[string]bool)
		for _, scope := range data.ScopeMetrics {
			for _, measurement := range scope.Metrics {
				found[measurement.Name] = true
				switch measurement.Name {
				case "tripgo.trips.created", "tripgo.trips.completed":
					sum, ok := measurement.Data.(metricdata.Sum[int64])
					if !ok || !sum.IsMonotonic || len(sum.DataPoints) != 1 {
						t.Fatalf("unexpected business counter: %+v", measurement)
					}
					want := int64(1)
					if measurement.Name == "tripgo.trips.created" {
						want = 2
					}
					if sum.DataPoints[0].Value != want || sum.DataPoints[0].Attributes.Len() != 0 {
						t.Errorf("unexpected counter point: %+v", sum.DataPoints[0])
					}
				case "tripgo.trips.active":
					gauge, ok := measurement.Data.(metricdata.Gauge[int64])
					if !ok || len(gauge.DataPoints) != 1 || gauge.DataPoints[0].Value != wantActive || gauge.DataPoints[0].Attributes.Len() != 0 {
						t.Errorf("unexpected active trips gauge: %+v", measurement)
					}
				}
			}
		}
		if len(found) != 3 {
			t.Errorf("business metrics = %v, want all 3", found)
		}
	}

	countErr = errors.New("database unavailable")
	var data metricdata.ResourceMetrics
	if err := reader.Collect(ctx, &data); err != nil {
		t.Fatal(err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name == "tripgo.trips.active" {
				t.Errorf("failed database read emitted a gauge value: %+v", measurement)
			}
		}
	}
}
