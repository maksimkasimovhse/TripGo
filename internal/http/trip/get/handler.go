package get

import (
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type Handler struct {
	getUsecase getUsecase
}

func New(get getUsecase) *Handler {
	return &Handler{getUsecase: get}
}

func (h *Handler) GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripScan, err := h.getUsecase.Get(r.Context(), tripID)

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("get trip %s: %v", tripID, err)
		respond.WriteProblemBody(w, r, status, problem)
		return
	}

	respond.WriteJSON(w, http.StatusOK, trip.ToAPITrip(tripScan))
}
