package get

import (
	"log/slog"
	"net/http"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type Handler struct {
	logger     *slog.Logger
	getUsecase getUsecase
}

func New(logger *slog.Logger, get getUsecase) *Handler {
	return &Handler{logger: logger, getUsecase: get}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripScan, err := h.getUsecase.Get(r.Context(), tripID)
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= http.StatusInternalServerError {
			h.logger.ErrorContext(r.Context(), "get trip failed", "error", err)
		}
		respond.WriteProblemBody(w, r, h.logger, status, problem)
		return
	}

	respond.WriteJSON(w, r, h.logger, http.StatusOK, trip.ToAPITrip(tripScan))
}
