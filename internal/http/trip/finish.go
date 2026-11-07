package trip

import (
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
)

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripId api.TripId) {
	tripForFinish, err := h.finishUsecase.Finish(r.Context(), tripId)

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("finish trip %s: %v", tripId, err)
		WriteProblemBody(w, r, status, problem)
		return
	}

	WriteJSON(w, http.StatusOK, toAPITrip(tripForFinish))
}
