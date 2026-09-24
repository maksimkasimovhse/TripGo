package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	api "github.com/maksimkasimovhse/TripGo.git/internal/generated"
)

type Handler struct {
	api.Unimplemented
	db *pgxpool.Pool
}

var _ api.ServerInterface = (*Handler)(nil)

func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.db.Ping(ctx); err != nil {
		log.Printf("db ping failed: %v", err)
		writeJSON(w, http.StatusServiceUnavailable, api.HealthResponse{Status: api.Unavailable})
		return
	}
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, api.HealthResponse{Status: api.Ok})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	err := godotenv.Load()
	if err != nil {
		log.Printf("failed to load .env: %v", err)
	}

	shutdownTimeout := 5 * time.Second
	if v := os.Getenv("SHUTDOWN_TIMEOUT"); v != "" {
		if timeV, err := time.ParseDuration(v); err == nil {
			shutdownTimeout = timeV
		} else {
			log.Printf("invalid SHUTDOWN_TIMEOUT %q, using default %s", v, shutdownTimeout)
		}
	}

	pool, err := pgxpool.New(context.Background(), os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatalf("failed to parse database config (DATABASE_URL): %v", err)
	}
	defer pool.Close()

	h := Handler{db: pool}

	router := chi.NewRouter()

	api.HandlerFromMux(&h, router)

	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       3 * time.Minute,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown timed out, some requests were not completed: %v", err)
	}
}
