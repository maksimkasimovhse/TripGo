package trip

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

type contextKey struct{}

var TripIDKey = contextKey{}

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

	tripID, ok := r.Context().Value(TripIDKey).(uuid.UUID)
	if !ok {
		tripID = uuid.New()
	}

	input := toCreateInput(req, tripID)
	tripForCreate, err := h.createUsecase.Create(r.Context(), input)

	var validationErr *trip.ValidationError
	if errors.As(err, &validationErr) {
		WriteInvalidRequest(w, r, validationErr.Error())
		return
	}

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
