package finish

import (
	"log/slog"
	"net/http"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type Handler struct {
	logger        *slog.Logger
	finishUsecase finishUsecase
}

func New(logger *slog.Logger, finish finishUsecase) *Handler {
	return &Handler{logger: logger, finishUsecase: finish}
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripForFinish, err := h.finishUsecase.Finish(r.Context(), tripID)
	if err != nil {
		status, problem := domainerror.Problem(err)
		if status >= http.StatusInternalServerError {
			h.logger.ErrorContext(r.Context(), "finish trip failed", "error", err)
		}
		respond.WriteProblemBody(w, r, h.logger, status, problem)
		return
	}

	respond.WriteJSON(w, r, h.logger, http.StatusOK, trip.ToAPITrip(tripForFinish))
}
