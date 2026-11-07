package container

import (
	"context"
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/config"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	httpmiddleware "github.com/maksimkasimovhse/TripGo/internal/http/middleware"
	handler "github.com/maksimkasimovhse/TripGo/internal/http/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/create"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/finish"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/get"
	txmanager "github.com/maksimkasimovhse/TripGo/internal/infra/postgres/transaction"
	repository "github.com/maksimkasimovhse/TripGo/internal/repository/trip"
	idempotencyUseCase "github.com/maksimkasimovhse/TripGo/internal/usecase/idempotency"
	createUC "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	finishUC "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
	getUC "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/get"
)

type Container struct {
	Server *http.Server
	pool   *pgxpool.Pool
}

func New(ctx context.Context, cfgEnv *config.Config) (Container, error) {
	cfg, err := pgxpool.ParseConfig(cfgEnv.DatabaseURL)
	if err != nil {
		return Container{}, fmt.Errorf("failed to parse database config (DATABASE_URL): %w", err)
	}
	cfg.MaxConns = cfgEnv.MaxConns
	cfg.MinConns = cfgEnv.MinConns
	cfg.MaxConnLifetime = cfgEnv.MaxConnLifetime
	cfg.ConnConfig.ConnectTimeout = cfgEnv.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return Container{}, fmt.Errorf("create pgx pool: %w", err)
	}

	ctxPing, pingCancel := context.WithTimeout(ctx, cfgEnv.ConnectTimeout)
	defer pingCancel()
	if err := pool.Ping(ctxPing); err != nil {
		pool.Close()
		return Container{}, fmt.Errorf("ping database: %w", err)
	}

	tripRepo := repository.NewTripRepository(pool, cfgEnv.QueryTimeout)
	historyRepo := repository.NewStatusHistoryRepository(pool, cfgEnv.QueryTimeout)
	idempotencyKeyRepo := repository.NewIdempotencyKeyRepository(pool, cfgEnv.QueryTimeout)

	txm := txmanager.New(pool, pgx.ReadCommitted, cfgEnv.QueryTimeout)

	createUC := createUC.New(txm, tripRepo, historyRepo)
	getUC := getUC.New(tripRepo)
	finishUC := finishUC.New(txm, tripRepo, historyRepo)
	idempotencyUC := idempotencyUseCase.New(idempotencyKeyRepo)

	createHandler := create.New(&createUC)
	finishHandler := finish.New(&finishUC)
	getHandler := get.New(&getUC)

	h := handler.New(createHandler, finishHandler, getHandler, pool, cfgEnv.QueryTimeout)

	decorator := httpmiddleware.NewWithIdempotency(
		h,
		&idempotencyUC,
		&getUC,
	)

	router := chi.NewRouter()
	router.Use(middleware.Recoverer)
	api.HandlerWithOptions(decorator,
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
	return Container{Server: srv, pool: pool}, nil
}

func (c *Container) Close() {
	c.pool.Close()
}
