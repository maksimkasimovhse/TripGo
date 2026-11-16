package trip

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
)

const MaxBodyBytes = 1 << 20

type Handler struct {
	logger       *slog.Logger
	create       createTrip
	finish       finishTrip
	get          getTrip
	readiness    readinessChecker
	readyTimeout time.Duration
}

func New(logger *slog.Logger, createHandler createTrip, finishHandler finishTrip, getHandler getTrip, readiness readinessChecker, readyTimeout time.Duration) *Handler {
	return &Handler{logger: logger, create: createHandler, finish: finishHandler, get: getHandler, readiness: readiness, readyTimeout: readyTimeout}
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
