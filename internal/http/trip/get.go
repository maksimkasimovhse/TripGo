package trip

import (
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
)

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripScan, err := h.getUsecase.Get(r.Context(), tripID)

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("get trip %s: %v", tripID, err)
		WriteProblemBody(w, r, status, problem)
		return
	}

	WriteJSON(w, http.StatusOK, toAPITrip(tripScan))
}
