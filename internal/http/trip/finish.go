package trip

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	var tripForFinish trip.Trip
	err := h.txm.Do(r.Context(), func(ctx context.Context) error {
		var err error
		finishedAt := time.Now().UTC()
		tripForFinish, err = h.tripRepo.Finish(ctx, tripId, finishedAt)
		if err != nil {
			return err
		}
		from := trip.StatusActive
		err = h.historyRepo.Add(ctx, tripId, &from, trip.StatusCompleted, finishedAt)
		if err != nil {
			return err
		}
		return nil
	})

	if errors.Is(err, trip.ErrTripNotFound) {
		WriteProblem(w, r, http.StatusNotFound, "trip_not_found", "Trip not found", "Trip was not found")
		return
	}
	if errors.Is(err, trip.ErrTripCompleted) {
		WriteProblem(w, r, http.StatusConflict, "trip_completed", "Trip completed", "Operation is not allowed for a completed trip")
		return
	}
	if err != nil {
		log.Printf("finish trip %s: %v", tripId, err)
		WriteProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	WriteJSON(w, http.StatusOK, toAPITrip(tripForFinish))
}
