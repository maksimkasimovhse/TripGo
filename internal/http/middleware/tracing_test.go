package middleware_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/maksimkasimovhse/TripGo/internal/http/middleware"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"
)

func TestTracing(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		status   int
		panic    bool
		wantCode codes.Code
	}{
		{name: "success", status: 200},
		{name: "client error", status: 404},
		{name: "server error", status: 500, wantCode: codes.Error},
		{name: "panic", status: 500, panic: true, wantCode: codes.Error},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := tracetest.NewSpanRecorder()
			provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder), sdktrace.WithSampler(sdktrace.AlwaysSample()))
			t.Cleanup(func() { _ = provider.Shutdown(t.Context()) })
			logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
			router := chi.NewRouter()
			router.Use(middleware.Recover(logger), middleware.Tracing(provider), middleware.RequestLogging(logger))
			router.Get("/trips/{tripId}", func(w http.ResponseWriter, r *http.Request) {
				_, child := provider.Tracer("test").Start(r.Context(), "child")
				child.End()
				if tt.panic {
					panic("test panic")
				}
				w.WriteHeader(tt.status)
			})

			req := httptest.NewRequest(http.MethodGet, "/trips/some-id", nil)
			req.Header.Set("traceparent", "00-0123456789abcdef0123456789abcdef-0123456789abcdef-01")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)

			spans := recorder.Ended()
			if len(spans) != 2 {
				t.Fatalf("spans = %d, want 2", len(spans))
			}
			child, server := spans[0], spans[1]
			if server.Name() != "GET /trips/{tripId}" || server.SpanKind() != trace.SpanKindServer {
				t.Errorf("server span = %s (%s)", server.Name(), server.SpanKind())
			}
			if server.SpanContext().TraceID().String() != "0123456789abcdef0123456789abcdef" || server.Parent().SpanID().String() != "0123456789abcdef" {
				t.Errorf("incoming trace context was not propagated: %+v", server.SpanContext())
			}
			if child.Parent().SpanID() != server.SpanContext().SpanID() {
				t.Error("child span is not attached to server span")
			}
			if response.Code != tt.status || server.Status().Code != tt.wantCode {
				t.Errorf("response = %d, span status = %v", response.Code, server.Status())
			}

			attrs := make(map[string]any)
			for _, attr := range server.Attributes() {
				attrs[string(attr.Key)] = attr.Value.AsInterface()
			}
			if attrs["http.route"] != "/trips/{tripId}" || attrs["http.response.status_code"] != int64(tt.status) {
				t.Errorf("HTTP span attributes = %+v", attrs)
			}
			if tt.wantCode == codes.Error && attrs["error.type"] == nil {
				t.Error("failed span is missing error.type")
			}
		})
	}
}
