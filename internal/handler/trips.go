package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

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

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	dec := json.NewDecoder(r.Body)
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

	trip := domain.Trip{
		ID:             uuid.New(),
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

	err := h.txm.Do(r.Context(), func(ctx context.Context) error {
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

func validCoordinates(c api.Coordinates) bool {
	return c.Latitude >= -90 && c.Latitude <= 90 &&
		c.Longitude >= -180 && c.Longitude <= 180
}
