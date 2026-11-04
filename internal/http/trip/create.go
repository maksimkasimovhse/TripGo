package trip

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, MaxBodyBytes))
	if err != nil {
		WriteInvalidRequest(w, r, "Request body is too large or unreadable")
		return
	}
	if msg := checkRequiredFields(body); msg != "" {
		WriteInvalidRequest(w, r, msg)
		return
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	var req api.TripData
	if err := dec.Decode(&req); err != nil {
		WriteInvalidRequest(w, r, "Request body is not valid JSON or contains unknown fields")
		return
	}

	if msg := validateTripData(req); msg != "" {
		WriteInvalidRequest(w, r, msg)
		return
	}

	tripID, ok := r.Context().Value(tripIDKey).(uuid.UUID)
	if !ok {
		tripID = uuid.New()
	}
	tripForCreate := trip.Trip{
		ID:             tripID,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  req.StartPoint.Latitude,
		StartLongitude: req.StartPoint.Longitude,
		EndLatitude:    req.EndPoint.Latitude,
		EndLongitude:   req.EndPoint.Longitude,
		Price:          req.Price,
		Status:         trip.StatusActive,
		StartedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	err = h.txm.Do(r.Context(), func(ctx context.Context) error {
		if err := h.tripRepo.Create(ctx, tripForCreate); err != nil {
			return err
		}
		return h.historyRepo.Add(ctx, tripForCreate.ID, nil, trip.StatusActive, tripForCreate.StartedAt)
	})

	if errors.Is(err, trip.ErrDriverBusy) {
		WriteProblem(w, r, http.StatusConflict, "driver_busy",
			"Driver busy", "Driver already has an active trip")
		return
	}
	if err != nil {
		log.Printf("create trip: %v", err)
		WriteProblem(w, r, http.StatusInternalServerError, "internal_error",
			"Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", tripLocation(tripForCreate.ID))
	WriteJSON(w, http.StatusCreated, toAPITrip(tripForCreate))
}
