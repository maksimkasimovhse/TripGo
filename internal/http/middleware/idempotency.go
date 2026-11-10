package middleware

import (
	"bytes"
	"context"
	"crypto/sha256"
	"io"
	"net/http"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (w *WithIdempotency) CreateTrip(rw http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	if params.IdempotencyKey == nil {
		w.ServerInterface.CreateTrip(rw, r, params)
		return
	}
	w.handleCreateTripIdempotent(rw, r, *params.IdempotencyKey, func(rw2 http.ResponseWriter, r2 *http.Request) {
		w.ServerInterface.CreateTrip(rw2, r2, params)
	})
}

func (w *WithIdempotency) handleCreateTripIdempotent(rw http.ResponseWriter, r *http.Request, key uuid.UUID, next func(http.ResponseWriter, *http.Request)) {
	body, err := io.ReadAll(http.MaxBytesReader(rw, r.Body, trip.MaxBodyBytes))
	if err != nil {
		respond.WriteInvalidRequest(rw, r, w.logger, "Request body is too large or unreadable")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := sha256.Sum256(body)

	result, err := w.usecase.Acquire(r.Context(), key, hash)
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= http.StatusInternalServerError {
			w.logger.ErrorContext(r.Context(), "acquire idempotency key failed", "error", err)
		}
		respond.WriteProblemBody(rw, r, w.logger, status, problem)
		return
	}

	if result.Replay {
		WriteCreateTripResponse(w.logger, rw, r, w.getter, http.StatusCreated, result.TripID)
		return
	}

	ctx := trip.WithTripID(r.Context(), result.TripID)
	rec := &statusRecorder{ResponseWriter: rw, status: http.StatusOK}

	func() {
		completed := false
		defer func() {
			if !completed {
				if err := w.usecase.Release(context.WithoutCancel(r.Context()), key); err != nil {
					w.logger.ErrorContext(r.Context(), "release idempotency key after panic failed", "error", err)
				}
			}
		}()
		next(rec, r.WithContext(ctx))
		completed = true
	}()

	if rec.status >= 400 {
		if err := w.usecase.Release(context.WithoutCancel(r.Context()), key); err != nil {
			w.logger.ErrorContext(r.Context(), "release idempotency key failed", "error", err)
		}
	} else {
		if err := w.usecase.Complete(context.WithoutCancel(r.Context()), key); err != nil {
			w.logger.ErrorContext(r.Context(), "complete idempotency key failed", "error", err)
		}
	}
}
