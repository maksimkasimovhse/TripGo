package handler

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

type Handler struct {
	api.Unimplemented
	txm          TxManager
	db           *pgxpool.Pool
	queryTimeout time.Duration
}

func New(txm TxManager, db *pgxpool.Pool, queryTimeout time.Duration) *Handler {
	return &Handler{txm: txm, db: db, queryTimeout: queryTimeout}
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

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
