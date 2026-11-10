package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	"go.opentelemetry.io/otel/trace/noop"
)

func TestQueryMetricsWithoutSampledTrace(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name      string
		err       error
		errorType string
	}{
		{name: "success"},
		{name: "no rows", err: pgx.ErrNoRows},
		{name: "constraint violation", err: &pgconn.PgError{Code: "23505"}, errorType: "23505"},
		{name: "canceled", err: context.Canceled, errorType: "*errors.errorString"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
			tracer, err := newQueryTracer(noop.NewTracerProvider(), provider)
			if err != nil {
				t.Fatal(err)
			}
			ctx := tracer.TraceQueryStart(context.Background(), nil, pgx.TraceQueryStartData{
				SQL: "SELECT id FROM trips WHERE id = $1", Args: []any{"private-id"},
			})
			tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: tt.err})

			var data metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &data); err != nil {
				t.Fatal(err)
			}
			if len(data.ScopeMetrics) != 1 || len(data.ScopeMetrics[0].Metrics) != 1 {
				t.Fatalf("unexpected metrics: %+v", data.ScopeMetrics)
			}
			measurement := data.ScopeMetrics[0].Metrics[0]
			histogram, ok := measurement.Data.(metricdata.Histogram[float64])
			if measurement.Name != "db.client.operation.duration" || measurement.Unit != "s" || !ok || len(histogram.DataPoints) != 1 {
				t.Fatalf("unexpected database histogram: %+v", measurement)
			}
			point := histogram.DataPoints[0]
			operation, _ := point.Attributes.Value("db.operation.name")
			collection, _ := point.Attributes.Value("db.collection.name")
			errorType, hasError := point.Attributes.Value("error.type")
			if point.Count != 1 || point.Sum < 0 || operation.AsString() != "SELECT" || collection.AsString() != "trips" || errorType.AsString() != tt.errorType || hasError != (tt.errorType != "") {
				t.Errorf("unexpected database point: %+v", point)
			}
		})
	}
}
