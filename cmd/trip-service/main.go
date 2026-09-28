package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/config"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/handler"
	"github.com/maksimkasimovhse/TripGo/internal/repository"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
)

func main() {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	cfgEnv, err := config.Load()
	if err != nil {
		log.Fatalf("Wrong parsing from .env: %v", err)
	}
	cfg, err := pgxpool.ParseConfig(cfgEnv.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to parse database config (DATABASE_URL): %v", err)
	}
	cfg.MaxConns = cfgEnv.MaxConns
	cfg.MinConns = cfgEnv.MinConns
	cfg.MaxConnLifetime = cfgEnv.MaxConnLifetime
	cfg.ConnConfig.ConnectTimeout = cfgEnv.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		log.Fatal(err)
	}
	ctxPing, pingCancel := context.WithTimeout(ctx, cfgEnv.ConnectTimeout)
	defer pingCancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		log.Fatalf("pgxpool does not reply: %v", err)
	}
	defer pool.Close()

	tripRepo := repository.NewTripRepository(pool, cfgEnv.QueryTimeout)
	historyRepo := repository.NewStatusHistoryRepository(pool, cfgEnv.QueryTimeout)

	txm := txmanager.New(pool, pgx.ReadCommitted, cfgEnv.QueryTimeout)
	h := handler.New(txm, tripRepo, historyRepo, pool, cfgEnv.QueryTimeout)

	router := chi.NewRouter()
	api.HandlerFromMux(h, router)

	srv := &http.Server{
		Addr:              cfgEnv.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfgEnv.ReadHeaderTimeout,
		ReadTimeout:       cfgEnv.ReadTimeout,
		WriteTimeout:      cfgEnv.WriteTimeout,
		IdleTimeout:       cfgEnv.IdleTimeout,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	}()

	<-ctx.Done()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfgEnv.ShutdownTimeout)
	defer shutdownCancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown timed out, some requests were not completed: %v", err)
	}
}
