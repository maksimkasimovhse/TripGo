package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/trace"
)

type queryTracer struct {
	tracer   *otelpgx.Tracer
	duration metric.Float64Histogram
}

type queryMeasurementKey struct{}

type queryMeasurement struct {
	started time.Time
	attrs   []attribute.KeyValue
}

func newQueryTracer(provider trace.TracerProvider, meters metric.MeterProvider) (*queryTracer, error) {
	duration, err := meters.Meter("tripgo/postgres").Float64Histogram("db.client.operation.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create database duration histogram: %w", err)
	}
	return &queryTracer{duration: duration, tracer: otelpgx.NewTracer(
		otelpgx.WithTracerProvider(provider),
		otelpgx.WithMeterProvider(noop.NewMeterProvider()),
		otelpgx.WithDisableSQLStatementInAttributes(),
	)}, nil
}

func (t *queryTracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	attrs := []attribute.KeyValue{attribute.String("db.operation.name", otelpgx.SQLOperationName(ctx, data.SQL))}
	if collection := queryCollection(data.SQL); collection != "" {
		attrs = append(attrs, attribute.String("db.collection.name", collection))
	}
	ctx = context.WithValue(ctx, queryMeasurementKey{}, queryMeasurement{started: time.Now(), attrs: attrs})
	ctx = t.tracer.TraceQueryStart(ctx, conn, data)
	span := trace.SpanFromContext(ctx)
	if !span.IsRecording() {
		return ctx
	}

	span.SetAttributes(attrs...)
	return ctx
}

func (t *queryTracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	measurement, measured := ctx.Value(queryMeasurementKey{}).(queryMeasurement)
	if data.Err != nil && !errors.Is(data.Err, pgx.ErrNoRows) {
		errorType := fmt.Sprintf("%T", data.Err)
		var pgErr *pgconn.PgError
		if errors.As(data.Err, &pgErr) {
			errorType = pgErr.Code
		}
		trace.SpanFromContext(ctx).SetAttributes(attribute.String("error.type", errorType))
		measurement.attrs = append(measurement.attrs, attribute.String("error.type", errorType))
	}
	if measured {
		t.duration.Record(ctx, time.Since(measurement.started).Seconds(), metric.WithAttributes(measurement.attrs...))
	}
	t.tracer.TraceQueryEnd(ctx, conn, data)
}

func queryCollection(query string) string {
	words := strings.Fields(query)
	for i := 0; i+1 < len(words); i++ {
		switch strings.ToUpper(words[i]) {
		case "FROM", "INTO", "UPDATE":
			name := strings.Trim(words[i+1], `"`)
			switch name {
			case "trips", "trip_status_history", "idempotency_keys":
				return name
			}
		}
	}
	return ""
}
