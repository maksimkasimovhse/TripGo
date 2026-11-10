package middleware

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"

	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
)

func Recover(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			state := &requestState{
				writer: chimiddleware.NewWrapResponseWriter(w, r.ProtoMajor),
				ctx:    r.Context(),
			}
			r = r.WithContext(context.WithValue(r.Context(), requestStateKey{}, state))

			defer func() {
				if panicValue := recover(); panicValue != nil {
					logger.ErrorContext(state.ctx, "request panicked",
						"error", fmt.Sprint(panicValue),
						"stack", string(debug.Stack()),
					)

					if state.writer.Status() == 0 {
						respond.WriteProblem(state.writer, r, logger, http.StatusInternalServerError,
							"internal_error", "Internal Server Error", "Internal server error")
					}
					if state.finish != nil {
						state.finish()
					}
				}
			}()

			next.ServeHTTP(state.writer, r)
		})
	}
}
