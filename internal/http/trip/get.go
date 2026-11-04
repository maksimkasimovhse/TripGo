package trip

import (
	"errors"
	"log"
	"net/http"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripScan, err := h.tripRepo.GetByID(r.Context(), tripId)
	if errors.Is(err, trip.ErrTripNotFound) {
		WriteProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
		return
	}
	if err != nil {
		log.Printf("get trip %s: %v", tripId, err)
		WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}
	WriteJSON(w, http.StatusOK, toAPITrip(tripScan))
}
