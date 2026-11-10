package telemetry

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestExporterTimeoutFromEnv(t *testing.T) {
	for _, signal := range []string{"traces", "metrics"} {
		t.Run(signal, func(t *testing.T) {
			clearTracingEnv(t)
			clearMetricsEnv(t)
			requestCanceled := make(chan struct{}, 1)
			receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					t.Error(err)
				}
				<-r.Context().Done()
				select {
				case requestCanceled <- struct{}{}:
				default:
				}
			}))
			defer receiver.Close()
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", httpProtobufProtocol)
			t.Setenv("OTEL_EXPORTER_OTLP_TIMEOUT", "20")
			t.Setenv("OTEL_BSP_EXPORT_TIMEOUT", "1000")
			t.Setenv("OTEL_METRIC_EXPORT_TIMEOUT", "1000")
			t.Setenv("OTEL_TRACES_SAMPLER", "always_on")

			var provider interface {
				ForceFlush(context.Context) error
				Shutdown(context.Context) error
			}
			if signal == "traces" {
				tracing, err := NewTracing(context.Background(), slog.New(slog.DiscardHandler))
				if err != nil {
					t.Fatal(err)
				}
				_, span := tracing.Tracer("test").Start(context.Background(), "timeout")
				span.End()
				provider = tracing
			} else {
				metrics, err := NewMetrics(context.Background())
				if err != nil {
					t.Fatal(err)
				}
				counters, err := NewTripMetrics(metrics)
				if err != nil {
					t.Fatal(err)
				}
				counters.Created.Add(context.Background(), 1)
				provider = metrics
			}
			defer func() {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				_ = provider.Shutdown(ctx)
			}()

			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			done := make(chan error, 1)
			go func() { done <- provider.ForceFlush(ctx) }()
			select {
			case <-requestCanceled:
				if ctx.Err() != nil {
					t.Error("request reached the collection deadline instead of the shorter exporter timeout")
				}
			case <-ctx.Done():
				t.Error("export ignored OTEL_EXPORTER_OTLP_TIMEOUT")
			}
			cancel()
			if err := <-done; err == nil {
				t.Error("export to a nonresponding receiver succeeded")
			}
		})
	}
}
