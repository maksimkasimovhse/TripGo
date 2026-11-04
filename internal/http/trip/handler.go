package trip

import (
	"context"
	"log"
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		log.Printf("db ping failed: %v", err)
		WriteJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	WriteJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
