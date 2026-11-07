package trip

import (
	"context"
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
)

type readinessChecker interface {
	Ping(ctx context.Context) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()

	if err := h.readiness.Ping(ctx); err != nil {
		log.Printf("db ping failed: %v", err)
		respond.WriteJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	respond.WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
