package trip

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	repository "github.com/maksimkasimovhse/TripGo/internal/repository/trip"
)

type Handler struct {
	txm                TxManager
	tripRepo           *repository.TripRepository
	historyRepo        *repository.StatusHistoryRepository
	idempotencyKeyRepo *repository.IdempotencyKeyRepository
	db                 *pgxpool.Pool
	queryTimeout       time.Duration //юзкейсы
}

func New(txm TxManager,
	tripRepo *repository.TripRepository,
	historyRepo *repository.StatusHistoryRepository,
	idempotencyKeyRepo *repository.IdempotencyKeyRepository,
	db *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{txm: txm, tripRepo: tripRepo, historyRepo: historyRepo, idempotencyKeyRepo: idempotencyKeyRepo, db: db, queryTimeout: queryTimeout}
}

type TxManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}

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
