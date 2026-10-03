package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/repository"
)

type ctxKey string

const tripIDKey ctxKey = "tripID"

type statusRecorder struct {
	http.ResponseWriter
	status int
}

type WithIdempotency struct {
	api.ServerInterface
	h *Handler
}

func NewWithIdempotency(h *Handler) *WithIdempotency {
	return &WithIdempotency{ServerInterface: h, h: h}
}

var _ api.ServerInterface = (*WithIdempotency)(nil)

func (w *WithIdempotency) CreateTrip(rw http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	if params.IdempotencyKey == nil {
		writeInvalidRequest(rw, r, "Idempotency-Key header is required")
		return
	}
	w.h.handleCreateTripIdempotent(rw, r, *params.IdempotencyKey, func(rw2 http.ResponseWriter, r2 *http.Request) {
		w.ServerInterface.CreateTrip(rw2, r2, params)
	})
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

func (h *Handler) handleCreateTripIdempotent(w http.ResponseWriter, r *http.Request, key uuid.UUID, next func(http.ResponseWriter, *http.Request)) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeInvalidRequest(w, r, "Request body is too large or unreadable")
		return
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	hash := sha256.Sum256(body)

	tripID := uuid.New()

	var inserted bool
	for attempt := 0; attempt < 2; attempt++ {
		inserted, err = h.idempotencyKeyRepo.TryInsert(r.Context(), key, hash, tripID)
		if err != nil {
			log.Println(err)
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		if inserted {
			break
		}
		
		savedTripID, savedHash, status, err := h.idempotencyKeyRepo.Get(r.Context(), key)
		if errors.Is(err, repository.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			log.Println(err)
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		if savedHash != hash {
			writeProblem(w, r, http.StatusConflict, "idempotency_key_conflict", "Idempotency Key Conflict", "Idempotency key conflict: request body does not match the original request")
			return
		}
		if status == repository.IdempotencyStatusProcessing {
			writeProblem(w, r, http.StatusConflict, "request_in_progress", "Request In Progress", "Исходный запрос с этим Idempotency-Key ещё обрабатывается")
			return
		}
		writeCreateTripResponse(w, r, h.tripRepo, http.StatusCreated, savedTripID)
		return
	}

	if !inserted {
		log.Printf("idempotency: failed to acquire key %s after retries", key)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	ctx := context.WithValue(r.Context(), tripIDKey, tripID)
	rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

	func() {
		completed := false
		defer func() {
			if !completed {
				if err := h.idempotencyKeyRepo.Delete(context.WithoutCancel(r.Context()), key); err != nil {
					log.Printf("idempotency delete after panic: %v", err)
				}
			}
		}()
		next(rec, r.WithContext(ctx))
		completed = true
	}()

	if rec.status >= 400 {
		if err := h.idempotencyKeyRepo.Delete(context.WithoutCancel(r.Context()), key); err != nil {
			log.Printf("idempotency delete: %v", err)
		}
	} else {
		if err := h.idempotencyKeyRepo.MarkCompleted(context.WithoutCancel(r.Context()), key); err != nil {
			log.Printf("idempotency mark completed: %v", err)
		}
	}
}
