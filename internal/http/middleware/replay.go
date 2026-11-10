package middleware

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

func WriteCreateTripResponse(logger *slog.Logger, w http.ResponseWriter, r *http.Request, getUsecase tripGetter, status int, tripID uuid.UUID) {
	tripScan, err := getUsecase.Get(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, trip.ErrTripNotFound) {
			logger.ErrorContext(r.Context(), "completed idempotency key has no trip", "trip_id", tripID.String(), "error", err)
			respond.WriteProblem(w, r, logger, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		logger.ErrorContext(r.Context(), "load trip for replay failed", "error", err)
		respond.WriteProblem(w, r, logger, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", triphttp.TripLocation(tripScan.ID))
	respond.WriteJSON(w, r, logger, status, triphttp.ToAPITrip(tripScan))
}
