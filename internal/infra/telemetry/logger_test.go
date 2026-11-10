package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"

	"go.opentelemetry.io/otel/trace"
)

const (
	httpProtobufProtocol = "http/protobuf"
	expectedServiceName  = "trip-service"
	grpcProtocol         = "grpc"
	createdMetricName    = "tripgo.trips.created"
)

func TestLoggerLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  int
	}{
		{name: "debug", level: slog.LevelDebug, want: 4},
		{name: "info", level: slog.LevelInfo, want: 3},
		{name: "warn", level: slog.LevelWarn, want: 2},
		{name: "error", level: slog.LevelError, want: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := newLogger(&output, tt.level)
			levels := []slog.Level{slog.LevelDebug, slog.LevelInfo, slog.LevelWarn, slog.LevelError}

			for _, level := range levels {
				logger.Log(context.Background(), level, "event")
			}

			lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
			if len(lines) != tt.want {
				t.Fatalf("records = %d, want %d", len(lines), tt.want)
			}
			for i, line := range lines {
				var record map[string]any
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				wantLevel := levels[len(levels)-tt.want+i].String()
				if record["level"] != wantLevel || record["service"] != expectedServiceName || record["msg"] != "event" {
					t.Errorf("unexpected record: %+v", record)
				}
			}
		})
	}
}

func TestLoggerSpanContext(t *testing.T) {
	t.Parallel()

	span := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID: trace.TraceID{1},
		SpanID:  trace.SpanID{2},
	})

	for _, withSpan := range []bool{false, true} {
		name := "without span"
		if withSpan {
			name = "with span"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := newLogger(&output, slog.LevelInfo).With("component", "test")
			ctx := context.Background()
			if withSpan {
				ctx = trace.ContextWithSpanContext(ctx, span)
			}

			logger.InfoContext(ctx, "event", "status", 200)

			var record map[string]any
			if err := json.Unmarshal(output.Bytes(), &record); err != nil {
				t.Fatalf("invalid JSON: %v", err)
			}
			if withSpan {
				if record["trace_id"] != span.TraceID().String() || record["span_id"] != span.SpanID().String() {
					t.Errorf("missing span correlation: %+v", record)
				}
			} else {
				if _, ok := record["trace_id"]; ok {
					t.Error("trace_id present without a span")
				}
				if _, ok := record["span_id"]; ok {
					t.Error("span_id present without a span")
				}
			}
			if record["component"] != "test" || record["service"] != expectedServiceName || record["status"] != float64(200) {
				t.Errorf("attributes lost: %+v", record)
			}
		})
	}
}
