package handler

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/repository"
)

type Handler struct {
	api.Unimplemented
	txm          TxManager
	tripRepo     *repository.TripRepository
	historyRepo  *repository.StatusHistoryRepository
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

func New(txm TxManager, tripRepo *repository.TripRepository, historyRepo *repository.StatusHistoryRepository, db *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{txm: txm, tripRepo: tripRepo, historyRepo: historyRepo, db: db, queryTimeout: queryTimeout}
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), h.queryTimeout)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		log.Printf("db ping failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}
