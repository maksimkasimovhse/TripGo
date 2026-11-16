package trip

import (
	"context"
	"net/http"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mocks_test.go -package=trip_test -mock_names=readinessChecker=MockReadinessChecker

type readinessChecker interface {
	Ping(ctx context.Context) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	respond.WriteJSON(w, r, h.logger, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.readyTimeout)
	defer cancel()

	if err := h.readiness.Ping(ctx); err != nil {
		h.logger.ErrorContext(r.Context(), "database readiness failed", "error", err)
		respond.WriteJSON(w, r, h.logger, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	respond.WriteJSON(w, r, h.logger, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
