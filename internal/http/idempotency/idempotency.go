package idempotency

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type ctxKey string

const tripIDKey ctxKey = "tripID"

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
		trip.WriteInvalidRequest(rw, r, "Idempotency-Key header is required")
		return
	}
	w.handleCreateTripIdempotent(rw, r, *params.IdempotencyKey, func(rw2 http.ResponseWriter, r2 *http.Request) {
		w.ServerInterface.CreateTrip(rw2, r2, params)
	})
}

func (w *WithIdempotency) handleCreateTripIdempotent(rw http.ResponseWriter, r *http.Request, key uuid.UUID, next func(http.ResponseWriter, *http.Request)) {
	body, err := io.ReadAll(http.MaxBytesReader(rw, r.Body, trip.MaxBodyBytes))
	if err != nil {
		trip.WriteInvalidRequest(rw, r, "Request body is too large or unreadable")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := sha256.Sum256(body)

	result, err := w.usecase.Acquire(r.Context(), key, hash)
	if err != nil {
		switch {
		case errors.Is(err, idempotency.ErrIdempotencyKeyConflict):
			trip.WriteProblem(rw, r, http.StatusConflict, "idempotency_key_conflict", "Idempotency Key Conflict", "Request body does not match the original request")

		case errors.Is(err, idempotency.ErrRequestInProgress):
			trip.WriteProblem(rw, r, http.StatusConflict, "request_in_progress", "Request In Progress", "Original request is still processing")

		default:
			log.Printf("acquire idempotency key: %v", err)
			trip.WriteProblem(rw, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		}
		return
	}

	if result.Replay {
		trip.WriteCreateTripResponse(rw, r, w.getter, http.StatusCreated, result.TripID)
		return
	}

	ctx := context.WithValue(r.Context(), trip.TripIDKey, result.TripID)
	rec := &statusRecorder{ResponseWriter: rw, status: http.StatusOK}

	func() {
		completed := false
		defer func() {
			if !completed {
				if err := w.usecase.Release(context.WithoutCancel(r.Context()), key); err != nil {
					log.Printf("idempotency delete after panic: %v", err)
				}
			}
		}()
		next(rec, r.WithContext(ctx))
		completed = true
	}()

	if rec.status >= 400 {
		if err := w.usecase.Release(context.WithoutCancel(r.Context()), key); err != nil {
			log.Printf("idempotency release: %v", err)
		}
	} else {
		if err := w.usecase.Complete(context.WithoutCancel(r.Context()), key); err != nil {
			log.Printf("idempotency complete: %v", err)
		}
	}
}
