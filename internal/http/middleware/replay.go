package middleware

import (
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

func WriteCreateTripResponse(w http.ResponseWriter, r *http.Request, getUsecase tripGetter, status int, tripID uuid.UUID) {
	tripScan, err := getUsecase.Get(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, trip.ErrTripNotFound) {
			log.Printf("idempotent response: trip %s marked completed but not found: %v", tripID, err)
			respond.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		log.Printf("load trip for idempotent response: %v", err)
		respond.WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", triphttp.TripLocation(tripScan.ID))
	respond.WriteJSON(w, status, triphttp.ToAPITrip(tripScan))
}
