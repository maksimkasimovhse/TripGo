package middleware_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/maksimkasimovhse/TripGo/internal/http/middleware"
)

func TestHTTPMetrics(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name   string
		status int
		panics bool
	}{
		{name: "success", status: 201},
		{name: "client error", status: 409},
		{name: "server error", status: 500},
		{name: "panic", status: 500, panics: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reader := sdkmetric.NewManualReader()
			provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
			t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
			router := chi.NewRouter()
			metrics, err := middleware.Metrics(router, provider)
			if err != nil {
				t.Fatal(err)
			}
			router.Use(middleware.Recover(slog.New(slog.DiscardHandler)), middleware.RequestLogging(slog.New(slog.DiscardHandler)), metrics)
			router.Post(testTripRoute, func(w http.ResponseWriter, r *http.Request) {
				assertActiveRequests(t, reader, 1)
				if tt.panics {
					panic("test panic")
				}
				w.WriteHeader(tt.status)
			})

			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/trips/private-id", nil))
			assertActiveRequests(t, reader, 0)

			var data metricdata.ResourceMetrics
			if err := reader.Collect(context.Background(), &data); err != nil {
				t.Fatal(err)
			}
			var found bool
			for _, scope := range data.ScopeMetrics {
				for _, measurement := range scope.Metrics {
					if measurement.Name != "http.server.request.duration" {
						continue
					}
					found = true
					histogram, ok := measurement.Data.(metricdata.Histogram[float64])
					if !ok || len(histogram.DataPoints) != 1 {
						t.Fatalf("unexpected histogram: %+v", measurement.Data)
					}
					point := histogram.DataPoints[0]
					route, _ := point.Attributes.Value("http.route")
					method, _ := point.Attributes.Value("http.request.method")
					status, _ := point.Attributes.Value("http.response.status_code")
					if measurement.Unit != "s" || point.Count != 1 || point.Sum < 0 || route.AsString() != testTripRoute || method.AsString() != "POST" || status.AsInt64() != int64(tt.status) {
						t.Errorf("unexpected HTTP measurement: %+v", measurement)
					}
				}
			}
			if !found {
				t.Error("HTTP duration histogram is missing")
			}
			if response.Code != tt.status {
				t.Errorf("response status = %d, want %d", response.Code, tt.status)
			}
		})
	}
}

func assertActiveRequests(t *testing.T, reader *sdkmetric.ManualReader, want int64) {
	t.Helper()
	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name != "http.server.active_requests" {
				continue
			}
			sum, ok := measurement.Data.(metricdata.Sum[int64])
			if !ok || sum.IsMonotonic || len(sum.DataPoints) != 1 {
				t.Fatalf("unexpected active requests counter: %+v", measurement.Data)
			}
			point := sum.DataPoints[0]
			route, _ := point.Attributes.Value("http.route")
			if point.Value != want || route.AsString() != testTripRoute || point.Attributes.Len() != 2 {
				t.Errorf("active requests = %+v, want %d with route template", point, want)
			}
			return
		}
	}
	t.Fatal("active HTTP requests metric is missing")
}

func TestHTTPMetricsUnknownRoutes(t *testing.T) {
	t.Parallel()

	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() { _ = provider.Shutdown(context.Background()) })
	router := chi.NewRouter()
	metrics, err := middleware.Metrics(router, provider)
	if err != nil {
		t.Fatal(err)
	}
	router.Use(metrics)
	router.Get("/known", func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(200) })
	for _, path := range []string{"/unknown/first-private-id", "/unknown/second-private-id"} {
		router.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	var data metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &data); err != nil {
		t.Fatal(err)
	}
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name != "http.server.request.duration" {
				continue
			}
			histogram, ok := measurement.Data.(metricdata.Histogram[float64])
			if !ok || len(histogram.DataPoints) != 1 {
				t.Fatalf("unknown paths created separate metric series: %+v", measurement)
			}
			point := histogram.DataPoints[0]
			route, _ := point.Attributes.Value("http.route")
			status, _ := point.Attributes.Value("http.response.status_code")
			if point.Count != 2 || route.AsString() != "unmatched" || status.AsInt64() != 404 {
				t.Errorf("unexpected unknown route measurement: %+v", point)
			}
			return
		}
	}
	t.Fatal("HTTP duration histogram is missing")
}
