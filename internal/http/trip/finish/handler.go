package finish

import (
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip"
)

type Handler struct {
	finishUsecase finishUsecase
}

func New(finish finishUsecase) *Handler {
	return &Handler{finishUsecase: finish}
}

func (h *Handler) FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId) {
	tripForFinish, err := h.finishUsecase.Finish(r.Context(), tripID)

	if err != nil {
		status, problem := domainerror.Problem(err)
		log.Printf("finish trip %s: %v", tripID, err)
		respond.WriteProblemBody(w, r, status, problem)
		return
	}

	respond.WriteJSON(w, http.StatusOK, trip.ToAPITrip(tripForFinish))
}
