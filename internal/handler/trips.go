package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

type WithIdempotency struct {
	api.ServerInterface
	Mw func(http.Handler) http.Handler
}

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

func (h *Handler) IdempotencyKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key, err := uuid.Parse(r.Header.Get("Idempotency-Key"))
		if err != nil {
			writeInvalidRequest(w, r, "Idempotency-Key header must be a UUID")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
		if err != nil {
			writeInvalidRequest(w, r, "Request body is too large or unreadable")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		hash := sha256.Sum256(body)

		tripID := uuid.New()
		inserted, err := h.idempotencyKeyRepo.TryInsert(r.Context(), key, hash, tripID)
		if err != nil {
			log.Println(err)
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		if !inserted {
			savedTripID, savedHash, err := h.idempotencyKeyRepo.Get(r.Context(), key)
			if err != nil {
				log.Printf("idempotency get: %v", err)
				writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
				return
			}
			if savedHash != hash {
				http.Error(w, "Idempotency key conflict: body mismatch", http.StatusConflict)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Cache-Lookup", "HIT - Idempotent Request")
			w.WriteHeader(http.StatusOK)
			fmt.Fprintf(w, `{"status":"success","message":"Дубликат запроса обработан","trip_id":"%s"}`, savedTripID)
			return
		}

		ctx := context.WithValue(r.Context(), tripIDKey, tripID)
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r.WithContext(ctx))

		if rec.status >= 400 {
			if err := h.idempotencyKeyRepo.Delete(context.WithoutCancel(r.Context()), key); err != nil {
				log.Printf("idempotency delete: %v", err)
			}
		}
	})

}

func toAPITrip(t domain.Trip) api.Trip {
	return api.Trip{
		Id:         t.ID,
		UserId:     t.UserID,
		DriverId:   t.DriverID,
		StartPoint: api.Coordinates{Latitude: t.StartLatitude, Longitude: t.StartLongitude},
		EndPoint:   api.Coordinates{Latitude: t.EndLatitude, Longitude: t.EndLongitude},
		Price:      t.Price,
		Status:     api.TripStatus(t.Status),
		StartedAt:  t.StartedAt,
		FinishedAt: t.FinishedAt,
	}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	trip, err := h.tripRepo.GetByID(r.Context(), tripId)
	if errors.Is(err, domain.ErrTripNotFound) {
		writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
		return
	}
	if err != nil {
		log.Printf("get trip %s: %v", tripId, err)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}
	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	var trip domain.Trip
	err := h.txm.Do(r.Context(), func(ctx context.Context) error {
		var err error
		finishedAt := time.Now().UTC()
		trip, err = h.tripRepo.Finish(ctx, tripId, finishedAt)
		if err != nil {
			return err
		}
		from := domain.StatusActive
		err = h.historyRepo.Add(ctx, tripId, &from, domain.StatusCompleted, finishedAt)
		if err != nil {
			return err
		}
		return nil
	})

	if errors.Is(err, domain.ErrTripNotFound) {
		writeProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
		return
	}
	if errors.Is(err, domain.ErrTripCompleted) {
		writeProblem(w, r, http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
		return
	}
	if err != nil {
		log.Printf("finish trip %s: %v", tripId, err)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	writeJSON(w, http.StatusOK, toAPITrip(trip))
}

func (w *WithIdempotency) CreateTrip(rw http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	w.Mw(http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.ServerInterface.CreateTrip(rw, r, params)
	})).ServeHTTP(rw, r)
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		writeInvalidRequest(w, r, "Request body is too large or unreadable")
		return
	}
	if msg := checkRequiredFields(body); msg != "" {
		writeInvalidRequest(w, r, msg)
		return
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	var req api.TripData
	if err := dec.Decode(&req); err != nil {
		writeInvalidRequest(w, r, "Request body is not valid JSON or contains unknown fields")
		return
	}

	if msg := validateTripData(req); msg != "" {
		writeInvalidRequest(w, r, msg)
		return
	}

	tripID, ok := r.Context().Value(tripIDKey).(uuid.UUID)
	if !ok {
		tripID = uuid.New()
	}
	trip := domain.Trip{
		ID:             tripID,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  req.StartPoint.Latitude,
		StartLongitude: req.StartPoint.Longitude,
		EndLatitude:    req.EndPoint.Latitude,
		EndLongitude:   req.EndPoint.Longitude,
		Price:          req.Price,
		Status:         domain.StatusActive,
		StartedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	err = h.txm.Do(r.Context(), func(ctx context.Context) error {
		if err := h.tripRepo.Create(ctx, trip); err != nil {
			return err
		}
		return h.historyRepo.Add(ctx, trip.ID, nil, domain.StatusActive, trip.StartedAt)
	})

	if errors.Is(err, domain.ErrDriverBusy) {
		writeProblem(w, r, http.StatusConflict, "driver_busy",
			"Driver busy", "Driver already has an active trip")
		return
	}
	if err != nil {
		log.Printf("create trip: %v", err)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error",
			"Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", "/api/v1/trips/"+trip.ID.String())
	writeJSON(w, http.StatusCreated, toAPITrip(trip))
}

func validateTripData(d api.TripData) string {
	switch {
	case d.UserId == uuid.Nil:
		return "user_id is required"
	case d.DriverId == uuid.Nil:
		return "driver_id is required"
	case !validCoordinates(d.StartPoint):
		return "start_point is out of range"
	case !validCoordinates(d.EndPoint):
		return "end_point is out of range"
	case d.Price < 0:
		return "price must be >= 0"
	}
	return ""
}

const maxBodyBytes = 1 << 20 // 1 МБ

func checkRequiredFields(body []byte) string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return "Request body must be a JSON object"
	}
	for _, f := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		if v, ok := top[f]; !ok || string(v) == "null" {
			return f + " is required"
		}
	}
	for _, p := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(top[p], &point); err != nil {
			return p + " must be an object"
		}
		for _, f := range []string{"latitude", "longitude"} {
			if v, ok := point[f]; !ok || string(v) == "null" {
				return p + "." + f + " is required"
			}
		}
	}
	return ""
}

func validCoordinates(c api.Coordinates) bool {
	return c.Latitude >= -90 && c.Latitude <= 90 &&
		c.Longitude >= -180 && c.Longitude <= 180
}
