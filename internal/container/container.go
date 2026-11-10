package container

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
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
	"github.com/maksimkasimovhse/TripGo/internal/infra/telemetry"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/idempotency"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	finishuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
	getuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/get"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

type Container struct {
	Server  *http.Server
	pool    *pgxpool.Pool
	tracing *sdktrace.TracerProvider
	metrics *sdkmetric.MeterProvider
}

func New(ctx context.Context, logger *slog.Logger, cfgEnv *config.Config) (Container, error) {
	tracing, err := telemetry.NewTracing(ctx, logger)
	if err != nil {
		return Container{}, fmt.Errorf("initialize tracing: %w", err)
	}

	resources := Container{tracing: tracing}
	ready := false
	defer func() {
		if ready {
			return
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfgEnv.ShutdownTimeout)
		defer cancel()
		if err := resources.Close(shutdownCtx); err != nil {
			logger.ErrorContext(shutdownCtx, "telemetry shutdown failed", "error", err)
		}
	}()

	resources.metrics, err = telemetry.NewMetrics(ctx)
	if err != nil {
		return Container{}, fmt.Errorf("initialize metrics: %w", err)
	}
	pool, err := postgres.NewPool(ctx, cfgEnv, tracing, resources.metrics)
	if err != nil {
		return Container{}, fmt.Errorf("initialize postgres: %w", err)
	}
	resources.pool = pool

	tripRepo := repository.NewTripRepository(pool, cfgEnv.QueryTimeout)
	historyRepo := repository.NewStatusHistoryRepository(pool, cfgEnv.QueryTimeout)
	idempotencyKeyRepo := repository.NewIdempotencyKeyRepository(pool, cfgEnv.QueryTimeout)
	tripMetrics, err := telemetry.NewTripMetrics(resources.metrics)
	if err != nil {
		return Container{}, err
	}
	if err := telemetry.ObserveActiveTrips(resources.metrics, tripRepo.CountActive, logger); err != nil {
		return Container{}, err
	}

	txm := transaction.New(pool, pgx.ReadCommitted, cfgEnv.QueryTimeout)

	createUC := createuc.New(logger, txm, tripRepo, historyRepo, tripMetrics.Created)
	getUC := getuc.New(tripRepo)
	finishUC := finishuc.New(logger, txm, tripRepo, historyRepo, tripMetrics.Completed)
	idempotencyUC := idempotency.New(idempotencyKeyRepo)

	createHandler := createhttp.New(logger, &createUC)
	finishHandler := finishhttp.New(logger, &finishUC)
	getHandler := gethttp.New(logger, &getUC)

	h := triphttp.New(logger, createHandler, finishHandler, getHandler, pool, cfgEnv.QueryTimeout)

	decorator := httpmiddleware.NewWithIdempotency(logger,
		h,
		&idempotencyUC,
		&getUC,
	)

	router := chi.NewRouter()
	metricsMiddleware, err := httpmiddleware.Metrics(router, resources.metrics)
	if err != nil {
		return Container{}, err
	}
	router.Use(httpmiddleware.Recover(logger), httpmiddleware.Tracing(tracing), httpmiddleware.RequestLogging(logger), metricsMiddleware)
	api.HandlerWithOptions(decorator,
		api.ChiServerOptions{
			BaseRouter:       router,
			ErrorHandlerFunc: triphttp.ParamErrorHandler(logger),
		})

	srv := &http.Server{
		Addr:              cfgEnv.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: cfgEnv.ReadHeaderTimeout,
		ReadTimeout:       cfgEnv.ReadTimeout,
		WriteTimeout:      cfgEnv.WriteTimeout,
		IdleTimeout:       cfgEnv.IdleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}
	resources.Server = srv
	ready = true
	return resources, nil
}

func (c *Container) Close(ctx context.Context) error {
	var metricsErr error
	if c.metrics != nil {
		metricsErr = c.metrics.Shutdown(ctx)
	}
	if c.pool != nil {
		c.pool.Close()
	}
	return errors.Join(metricsErr, c.tracing.Shutdown(ctx))
}
