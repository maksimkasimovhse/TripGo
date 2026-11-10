package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
)

type requestStateKey struct{}

type requestState struct {
	writer chimiddleware.WrapResponseWriter
	ctx    context.Context
	finish func()
}

const unmatchedRoute = "unmatched"

func RequestLogging(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			started := time.Now()
			state, ok := r.Context().Value(requestStateKey{}).(*requestState)
			writer := w
			if !ok {
				state = &requestState{writer: chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor)}
				writer = state.writer
			}
			state.ctx = r.Context()

			state.finish = func() {
				status := state.writer.Status()
				if status == 0 {
					status = http.StatusOK
				}

				level := slog.LevelInfo
				if status >= http.StatusInternalServerError {
					level = slog.LevelError
				}

				logger.Log(r.Context(), level, "request handled",
					"method", r.Method,
					"route", requestRoute(r),
					"status", status,
					"duration_ms", time.Since(started).Milliseconds(),
				)
			}

			next.ServeHTTP(writer, r)
			state.finish()
		})
	}
}

func requestRoute(r *http.Request) string {
	if routeContext := chi.RouteContext(r.Context()); routeContext != nil {
		if route := routeContext.RoutePattern(); route != "" {
			return route
		}
	}

	return unmatchedRoute
}
