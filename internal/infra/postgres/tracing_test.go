package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

const testTripTable = "trips"

func TestQueryTracing(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		query      string
		operation  string
		collection string
	}{
		{"SELECT id FROM trips WHERE id = $1", "SELECT", testTripTable},
		{"INSERT INTO trip_status_history (trip_id) VALUES ($1)", "INSERT", "trip_status_history"},
		{"UPDATE trips SET status = $1", "UPDATE", testTripTable},
		{"DELETE FROM idempotency_keys WHERE id = $1", "DELETE", "idempotency_keys"},
	} {
		t.Run(tt.operation+" "+tt.collection, func(t *testing.T) {
			t.Parallel()

			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder), sdktrace.WithSampler(sdktrace.AlwaysSample()))
			t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
			ctx, parent := provider.Tracer("test").Start(context.Background(), "HTTP")
			defer parent.End()
			tracer, err := newQueryTracer(provider, noop.NewMeterProvider())
			if err != nil {
				t.Fatal(err)
			}

			ctx = tracer.TraceQueryStart(ctx, nil, pgx.TraceQueryStartData{SQL: tt.query, Args: []any{"private"}})
			tracer.TraceQueryEnd(ctx, nil, pgx.TraceQueryEndData{Err: &pgconn.PgError{Code: "23505", Message: "duplicate key"}})

			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("SQL spans = %d, want 1", len(spans))
			}
			span := spans[0]
			if span.Parent().SpanID() != parent.SpanContext().SpanID() || span.Status().Code != codes.Error {
				t.Errorf("unexpected SQL span: parent=%v status=%v", span.Parent(), span.Status())
			}

			attrs := make(map[string]any)
			for _, attr := range span.Attributes() {
				attrs[string(attr.Key)] = attr.Value.AsInterface()
			}
			if attrs["db.operation.name"] != tt.operation || attrs["db.collection.name"] != tt.collection || attrs["error.type"] != "23505" {
				t.Errorf("SQL attributes = %+v", attrs)
			}
			if attrs["db.query.text"] != nil || attrs["pgx.query.parameters"] != nil {
				t.Error("SQL text or parameters leaked into span")
			}
		})
	}
}
