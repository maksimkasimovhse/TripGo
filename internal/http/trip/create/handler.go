package create

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"

	"github.com/google/uuid"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type Handler struct {
	createUsecase createUsecase
}

func New(create createUsecase) *Handler {
	return &Handler{createUsecase: create}
}

func (h *Handler) CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, trip.MaxBodyBytes))
	if err != nil {
		respond.WriteInvalidRequest(w, r, "Request body is too large or unreadable")
		return
	}
	if msg := CheckRequiredFields(body); msg != "" {
		respond.WriteInvalidRequest(w, r, msg)
		return
	}

	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()

	var req api.TripData
	if err := dec.Decode(&req); err != nil {
		respond.WriteInvalidRequest(w, r, "Request body is not valid JSON or contains unknown fields")
		return
	}

	tripID, ok := trip.TripIDFromContext(r.Context())
	if !ok {
		tripID = uuid.New()
	}

	input := ToCreateInput(req, tripID)
	tripForCreate, err := h.createUsecase.Create(r.Context(), input)

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("create trip: %v", err)
		respond.WriteProblemBody(w, r, status, problem)
		return
	}

	w.Header().Set("Location", trip.TripLocation(tripForCreate.ID))
	respond.WriteJSON(w, http.StatusCreated, trip.ToAPITrip(tripForCreate))
}
