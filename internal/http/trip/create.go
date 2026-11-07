package trip

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
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

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("create trip: %v", err)
		WriteProblemBody(w, r, status, problem)
		return
	}

	w.Header().Set("Location", tripLocation(tripForCreate.ID))
	WriteJSON(w, http.StatusCreated, toAPITrip(tripForCreate))
}
