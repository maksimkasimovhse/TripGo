package telemetry

import (
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	collectortrace "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type traceReceiver struct {
	collectortrace.UnimplementedTraceServiceServer
	requests chan *collectortrace.ExportTraceServiceRequest
}

func (r *traceReceiver) Export(_ context.Context, request *collectortrace.ExportTraceServiceRequest) (*collectortrace.ExportTraceServiceResponse, error) {
	r.requests <- request
	return &collectortrace.ExportTraceServiceResponse{}, nil
}

func TestTracingGRPCExport(t *testing.T) {
	clearTracingEnv(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	receiver := &traceReceiver{requests: make(chan *collectortrace.ExportTraceServiceRequest, 1)}
	server := grpc.NewServer()
	collectortrace.RegisterTraceServiceServer(server, receiver)
	go func() { _ = server.Serve(listener) }()
	defer server.Stop()
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "grpc")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://"+listener.Addr().String())
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")

	provider, err := NewTracing(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	_, span := provider.Tracer("test").Start(context.Background(), "grpc span")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case request := <-receiver.requests:
		if len(request.ResourceSpans) != 1 || len(request.ResourceSpans[0].ScopeSpans) != 1 || len(request.ResourceSpans[0].ScopeSpans[0].Spans) != 1 {
			t.Fatalf("unexpected exported spans: %v", request.ResourceSpans)
		}
		if got := request.ResourceSpans[0].ScopeSpans[0].Spans[0].Name; got != "grpc span" {
			t.Errorf("span name = %q, want grpc span", got)
		}
	default:
		t.Fatal("no OTLP gRPC export received")
	}
}

func TestTracingHTTPExport(t *testing.T) {
	clearTracingEnv(t)
	requests := make(chan *collectortrace.ExportTraceServiceRequest, 1)
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/traces" || r.Header.Get("Content-Type") != "application/x-protobuf" {
			t.Errorf("unexpected OTLP request: %s %s", r.URL.Path, r.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		request := new(collectortrace.ExportTraceServiceRequest)
		if err := proto.Unmarshal(body, request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		requests <- request
		w.Header().Set("Content-Type", "application/x-protobuf")
	}))
	defer receiver.Close()
	t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", "http/protobuf")
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", receiver.URL)
	t.Setenv("OTEL_TRACES_SAMPLER", "always_on")

	provider, err := NewTracing(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	_, span := provider.Tracer("test").Start(context.Background(), "exported span")
	span.End()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := provider.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}

	select {
	case request := <-requests:
		if len(request.ResourceSpans) != 1 {
			t.Fatalf("resource spans = %d, want 1", len(request.ResourceSpans))
		}
		resourceSpans := request.ResourceSpans[0]
		attrs := make(map[string]string)
		for _, attr := range resourceSpans.Resource.Attributes {
			attrs[attr.Key] = attr.Value.GetStringValue()
		}
		for key, want := range map[string]string{
			"service.name": "trip-service", "service.version": Version,
			"service.namespace": "tripgo", "deployment.environment": "local",
		} {
			if attrs[key] != want {
				t.Errorf("resource %s = %q, want %q", key, attrs[key], want)
			}
		}
		if len(resourceSpans.ScopeSpans) != 1 || len(resourceSpans.ScopeSpans[0].Spans) != 1 || resourceSpans.ScopeSpans[0].Spans[0].Name != "exported span" {
			t.Errorf("unexpected exported spans: %v", resourceSpans.ScopeSpans)
		}
	default:
		t.Fatal("shutdown did not export the buffered span")
	}
}

func TestTracingWithoutCollector(t *testing.T) {
	for _, protocol := range []string{"grpc", "http/protobuf"} {
		t.Run(protocol, func(t *testing.T) {
			clearTracingEnv(t)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol)
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
			t.Setenv("OTEL_TRACES_SAMPLER", "always_on")

			provider, err := NewTracing(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
			if err != nil {
				t.Fatalf("collector availability prevented startup: %v", err)
			}
			ctx, span := provider.Tracer("test").Start(context.Background(), "request")
			if !span.IsRecording() || ctx.Err() != nil {
				t.Error("tracing context is not usable without collector")
			}
			span.End()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			if err := provider.ForceFlush(shutdownCtx); err == nil {
				t.Error("export to unavailable collector unexpectedly succeeded")
			}
			_ = provider.Shutdown(context.Background())
		})
	}
}

func TestTracingSamplingOff(t *testing.T) {
	clearTracingEnv(t)
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
	t.Setenv("OTEL_TRACES_SAMPLER", "always_off")
	provider, err := NewTracing(context.Background(), slog.New(slog.NewJSONHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	_, span := provider.Tracer("test").Start(context.Background(), "not exported")
	if span.IsRecording() {
		t.Error("always_off sampler recorded a span")
	}
	span.End()
	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func clearTracingEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_TRACES_PROTOCOL", "OTEL_EXPORTER_OTLP_TRACES_ENDPOINT",
		"OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_TRACES_HEADERS",
		"OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_TRACES_CERTIFICATE",
		"OTEL_EXPORTER_OTLP_INSECURE", "OTEL_EXPORTER_OTLP_TRACES_INSECURE",
		"OTEL_TRACES_SAMPLER_ARG", "OTEL_EXPORTER_OTLP_PROTOCOL",
	} {
		t.Setenv(key, "")
	}
}
