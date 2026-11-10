package middleware

import (
	"net/http"
	"strconv"

	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

func Tracing(provider trace.TracerProvider) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			writer := chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)
			span := trace.SpanFromContext(r.Context())
			if state, ok := r.Context().Value(requestStateKey{}).(*requestState); ok {
				state.ctx = r.Context()
			}

			defer func() {
				route := requestRoute(r)
				span.SetName(r.Method + " " + route)
				if route != "unmatched" {
					span.SetAttributes(attribute.String("http.route", route))
				}

				status := writer.Status()
				if panicValue := recover(); panicValue != nil {
					if status == 0 {
						status = http.StatusInternalServerError
					}
					span.SetAttributes(attribute.Int("http.response.status_code", status), attribute.String("error.type", "panic"))
					span.SetStatus(codes.Error, "request panicked")
					panic(panicValue)
				}
				if status >= http.StatusInternalServerError {
					span.SetAttributes(attribute.String("error.type", strconv.Itoa(status)))
					span.SetStatus(codes.Error, http.StatusText(status))
				}
			}()

			next.ServeHTTP(writer, r)
		})

		return otelhttp.NewHandler(handler, "HTTP",
			otelhttp.WithTracerProvider(provider),
			otelhttp.WithMeterProvider(noop.NewMeterProvider()),
			otelhttp.WithPropagators(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{})),
		)
	}
}
