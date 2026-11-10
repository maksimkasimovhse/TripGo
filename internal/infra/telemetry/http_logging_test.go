package telemetry

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/maksimkasimovhse/TripGo/internal/http/middleware"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestHTTPLogSpanCorrelation(t *testing.T) {
	t.Parallel()

	for _, panics := range []bool{false, true} {
		name := "normal request"
		if panics {
			name = "panic"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := newLogger(&output, slog.LevelInfo)
			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder), sdktrace.WithSampler(sdktrace.AlwaysSample()))
			t.Cleanup(func() { _ = provider.Shutdown(t.Context()) })
			router := chi.NewRouter()
			router.Use(middleware.Recover(logger), middleware.Tracing(provider), middleware.RequestLogging(logger))
			router.Get("/test", func(w http.ResponseWriter, r *http.Request) {
				if panics {
					panic("test panic")
				}
				w.WriteHeader(http.StatusNoContent)
			})

			router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/test", nil))
			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("spans = %d, want 1", len(spans))
			}
			span := spans[0].SpanContext()

			lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte("\n"))
			want := 1
			if panics {
				want = 2
			}
			if len(lines) != want {
				t.Fatalf("records = %d, want %d", len(lines), want)
			}
			for _, line := range lines {
				var record map[string]any
				if err := json.Unmarshal(line, &record); err != nil {
					t.Fatal(err)
				}
				if record["trace_id"] != span.TraceID().String() || record["span_id"] != span.SpanID().String() {
					t.Errorf("span correlation missing: %+v", record)
				}
			}
		})
	}
}
