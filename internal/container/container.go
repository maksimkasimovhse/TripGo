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
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	httpmiddleware "github.com/maksimkasimovhse/TripGo/internal/http/middleware"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
	createhttp "github.com/maksimkasimovhse/TripGo/internal/http/trip/create"
	finishhttp "github.com/maksimkasimovhse/TripGo/internal/http/trip/finish"
	gethttp "github.com/maksimkasimovhse/TripGo/internal/http/trip/get"
	"github.com/maksimkasimovhse/TripGo/internal/infra/postgres"
	"github.com/maksimkasimovhse/TripGo/internal/infra/postgres/transaction"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/idempotency"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	finishuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
	getuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/get"
)

type Container struct {
	Server *http.Server
	pool   *pgxpool.Pool
}

func New(ctx context.Context, cfgEnv *config.Config) (Container, error) {
	pool, err := postgres.NewPool(ctx, cfgEnv)
	if err != nil {
		return Container{}, fmt.Errorf("initialize postgres: %w", err)
	}

	tripRepo := repository.NewTripRepository(pool, cfgEnv.QueryTimeout)
	historyRepo := repository.NewStatusHistoryRepository(pool, cfgEnv.QueryTimeout)
	idempotencyKeyRepo := repository.NewIdempotencyKeyRepository(pool, cfgEnv.QueryTimeout)

	txm := transaction.New(pool, pgx.ReadCommitted, cfgEnv.QueryTimeout)

	createUC := createuc.New(txm, tripRepo, historyRepo)
	getUC := getuc.New(tripRepo)
	finishUC := finishuc.New(txm, tripRepo, historyRepo)
	idempotencyUC := idempotency.New(idempotencyKeyRepo)

	createHandler := createhttp.New(&createUC)
	finishHandler := finishhttp.New(&finishUC)
	getHandler := gethttp.New(&getUC)

	h := triphttp.New(createHandler, finishHandler, getHandler, pool, cfgEnv.QueryTimeout)

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
			ErrorHandlerFunc: triphttp.ParamErrorHandler,
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
