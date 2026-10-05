package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/config"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/handler"
	"github.com/maksimkasimovhse/TripGo/internal/repository"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
)

func Run() error {
	ctx, stop := signal.NotifyContext(
		context.Background(),
		syscall.SIGINT,
		syscall.SIGTERM,
	)
	defer stop()

	cfgEnv, err := config.Load()
	if err != nil {
		return fmt.Errorf("wrong parsing from .env: %w", err)
	}
	cfg, err := pgxpool.ParseConfig(cfgEnv.DatabaseURL)
	if err != nil {
		return fmt.Errorf("failed to parse database config (DATABASE_URL): %w", err)
	}
	cfg.MaxConns = cfgEnv.MaxConns
	cfg.MinConns = cfgEnv.MinConns
	cfg.MaxConnLifetime = cfgEnv.MaxConnLifetime
	cfg.ConnConfig.ConnectTimeout = cfgEnv.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("create pgx pool: %w", err)
	}
	defer pool.Close()

	ctxPing, pingCancel := context.WithTimeout(ctx, cfgEnv.ConnectTimeout)
	defer pingCancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return fmt.Errorf("ping database: %w", err)
	}

	tripRepo := repository.NewTripRepository(pool, cfgEnv.QueryTimeout)
	historyRepo := repository.NewStatusHistoryRepository(pool, cfgEnv.QueryTimeout)
	idempotencyKeyRepo := repository.NewIdempotencyKeyRepository(pool, cfgEnv.QueryTimeout)

	txm := txmanager.New(pool, pgx.ReadCommitted, cfgEnv.QueryTimeout)
	h := handler.New(txm, tripRepo, historyRepo, idempotencyKeyRepo, pool, cfgEnv.QueryTimeout)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	api.HandlerWithOptions(handler.NewWithIdempotency(h),
		api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: handler.ParamErrorHandler,
		})

	srv := &http.Server{
		Addr:              cfgEnv.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfgEnv.ReadHeaderTimeout,
		ReadTimeout:       cfgEnv.ReadTimeout,
		WriteTimeout:      cfgEnv.WriteTimeout,
		IdleTimeout:       cfgEnv.IdleTimeout,
	}

	srvErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	select {
	case err := <-srvErr:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfgEnv.ShutdownTimeout)
	defer shutdownCancel()

	if err = srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	return nil
}
