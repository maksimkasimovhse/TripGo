package trip

import (
	"net/http"
	"time"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

const MaxBodyBytes = 1 << 20

type Handler struct {
	create       createTrip
	finish       finishTrip
	get          getTrip
	readiness    readinessChecker
	readyTimeout time.Duration
}

func New(createHandler createTrip, finishHandler finishTrip, getHandler getTrip, readiness readinessChecker, readyTimeout time.Duration) *Handler {
	return &Handler{create: createHandler, finish: finishHandler, get: getHandler, readiness: readiness, readyTimeout: readyTimeout}
}

func (h *Handler) CreateTrip(
	w http.ResponseWriter,
	r *http.Request,
	params api.CreateTripParams,
) {
	h.create.CreateTrip(w, r, params)
}

func (h *Handler) GetTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	h.get.GetTrip(w, r, tripID)
}

func (h *Handler) FinishTrip(
	w http.ResponseWriter,
	r *http.Request,
	tripID api.TripId,
) {
	h.finish.FinishTrip(w, r, tripID)
}

var _ api.ServerInterface = (*Handler)(nil)
