package telemetry

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	collectormetrics "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	"google.golang.org/grpc"
	"google.golang.org/protobuf/proto"
)

type metricsReceiver struct {
	collectormetrics.UnimplementedMetricsServiceServer
	requests chan *collectormetrics.ExportMetricsServiceRequest
}

func (r *metricsReceiver) Export(_ context.Context, request *collectormetrics.ExportMetricsServiceRequest) (*collectormetrics.ExportMetricsServiceResponse, error) {
	r.requests <- request
	return &collectormetrics.ExportMetricsServiceResponse{}, nil
}

func TestMetricsExport(t *testing.T) {
	for _, protocol := range []string{grpcProtocol, httpProtobufProtocol} {
		t.Run(protocol, func(t *testing.T) {
			clearMetricsEnv(t)
			requests := make(chan *collectormetrics.ExportMetricsServiceRequest, 1)
			var endpoint string
			if protocol == grpcProtocol {
				listener, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				server := grpc.NewServer()
				collectormetrics.RegisterMetricsServiceServer(server, &metricsReceiver{requests: requests})
				go func() { _ = server.Serve(listener) }()
				defer server.Stop()
				endpoint = "http://" + listener.Addr().String()
			} else {
				receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path != "/v1/metrics" || r.Header.Get("Content-Type") != "application/x-protobuf" {
						t.Errorf("unexpected OTLP metrics request: %s %s", r.URL.Path, r.Header.Get("Content-Type"))
					}
					body, err := io.ReadAll(r.Body)
					if err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					request := new(collectormetrics.ExportMetricsServiceRequest)
					if err := proto.Unmarshal(body, request); err != nil {
						t.Error(err)
						w.WriteHeader(400)
						return
					}
					requests <- request
					w.Header().Set("Content-Type", "application/x-protobuf")
				}))
				defer receiver.Close()
				endpoint = receiver.URL
			}
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol)
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", endpoint)

			provider, err := NewMetrics(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			counters, err := NewTripMetrics(provider)
			if err != nil {
				t.Fatal(err)
			}
			counters.Created.Add(context.Background(), 3)
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := provider.Shutdown(ctx); err != nil {
				t.Fatal(err)
			}

			select {
			case request := <-requests:
				if len(request.ResourceMetrics) != 1 {
					t.Fatalf("resource metrics = %d, want 1", len(request.ResourceMetrics))
				}
				resource := request.ResourceMetrics[0]
				attrs := make(map[string]string)
				for _, attr := range resource.Resource.Attributes {
					attrs[attr.Key] = attr.Value.GetStringValue()
				}
				if attrs["service.name"] != expectedServiceName || attrs["service.namespace"] != "tripgo" || attrs["service.version"] != Version || attrs["deployment.environment"] != "local" {
					t.Errorf("unexpected resource: %v", attrs)
				}
				if len(resource.ScopeMetrics) != 1 || len(resource.ScopeMetrics[0].Metrics) != 1 {
					t.Fatalf("unexpected exported metrics: %v", resource.ScopeMetrics)
				}
				measurement := resource.ScopeMetrics[0].Metrics[0]
				if measurement.Name != createdMetricName || measurement.GetSum() == nil || len(measurement.GetSum().DataPoints) != 1 || measurement.GetSum().DataPoints[0].GetAsInt() != 3 {
					t.Errorf("unexpected exported metric: %v", measurement)
				}
			default:
				t.Fatal("shutdown did not export the recorded metric")
			}
		})
	}
}

func TestMetricsWithoutCollector(t *testing.T) {
	for _, protocol := range []string{grpcProtocol, httpProtobufProtocol} {
		t.Run(protocol, func(t *testing.T) {
			clearMetricsEnv(t)
			t.Setenv("OTEL_EXPORTER_OTLP_PROTOCOL", protocol)
			t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "http://127.0.0.1:1")
			provider, err := NewMetrics(context.Background())
			if err != nil {
				t.Fatalf("collector availability prevented startup: %v", err)
			}
			counters, err := NewTripMetrics(provider)
			if err != nil {
				t.Fatal(err)
			}
			counters.Created.Add(context.Background(), 1)
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			_ = provider.Shutdown(ctx)
		})
	}
}

func clearMetricsEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{
		"OTEL_EXPORTER_OTLP_METRICS_ENDPOINT", "OTEL_EXPORTER_OTLP_METRICS_PROTOCOL",
		"OTEL_EXPORTER_OTLP_HEADERS", "OTEL_EXPORTER_OTLP_METRICS_HEADERS",
		"OTEL_EXPORTER_OTLP_CERTIFICATE", "OTEL_EXPORTER_OTLP_METRICS_CERTIFICATE",
		"OTEL_EXPORTER_OTLP_INSECURE", "OTEL_EXPORTER_OTLP_METRICS_INSECURE",
		"OTEL_EXPORTER_OTLP_METRICS_TEMPORALITY_PREFERENCE",
		"OTEL_EXPORTER_OTLP_METRICS_DEFAULT_HISTOGRAM_AGGREGATION",
	} {
		t.Setenv(key, "")
	}
	t.Setenv("OTEL_METRIC_EXPORT_INTERVAL", "60000")
}
