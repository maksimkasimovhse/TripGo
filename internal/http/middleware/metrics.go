package middleware

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func Metrics(router chi.Router, provider metric.MeterProvider) (func(http.Handler) http.Handler, error) {
	meter := provider.Meter("tripgo/http")
	duration, err := meter.Float64Histogram("http.server.request.duration",
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(.005, .01, .025, .05, .075, .1, .25, .5, .75, 1, 2.5, 5, 7.5, 10),
	)
	if err != nil {
		return nil, fmt.Errorf("create HTTP duration histogram: %w", err)
	}
	active, err := meter.Int64UpDownCounter("http.server.active_requests", metric.WithUnit("{request}"))
	if err != nil {
		return nil, fmt.Errorf("create active HTTP requests counter: %w", err)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			route := "unmatched"
			match := chi.NewRouteContext()
			if router.Match(match, r.Method, r.URL.Path) {
				route = match.RoutePattern()
			}
			attrs := []attribute.KeyValue{
				attribute.String("http.request.method", r.Method),
				attribute.String("http.route", route),
			}
			active.Add(r.Context(), 1, metric.WithAttributes(attrs...))

			defer func() {
				panicValue := recover()
				status := writer.Status()
				if status == 0 {
					status = http.StatusOK
					if panicValue != nil {
						status = http.StatusInternalServerError
					}
				}
				active.Add(r.Context(), -1, metric.WithAttributes(attrs...))
				attrs = append(attrs, attribute.Int("http.response.status_code", status))
				duration.Record(r.Context(), time.Since(started).Seconds(), metric.WithAttributes(attrs...))
				if panicValue != nil {
					panic(panicValue)
				}
			}()

			next.ServeHTTP(writer, r)
		})
	}, nil
}
