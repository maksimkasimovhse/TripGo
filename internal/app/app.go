package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/maksimkasimovhse/TripGo/internal/config"
	"github.com/maksimkasimovhse/TripGo/internal/container"
	"github.com/maksimkasimovhse/TripGo/internal/infra/telemetry"
)

func Run(ctx context.Context) error {
	cfgEnv, err := config.Load()
	if err != nil {
		return fmt.Errorf("wrong parsing from .env: %w", err)
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfgEnv.LogLevel)); err != nil {
		return fmt.Errorf("parse log level: %w", err)
	}
	logger := telemetry.NewLogger(level)

	containerApp, err := container.New(ctx, logger, &cfgEnv)
	if err != nil {
		return fmt.Errorf("build application: %w", err)
	}
	logger.InfoContext(ctx, "service starting", "addr", cfgEnv.HTTPAddr)

	srvErr := make(chan error, 1)
	go func() {
		if err := containerApp.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			srvErr <- err
		}
	}()

	var serveErr error
	select {
	case err := <-srvErr:
		serveErr = fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.WithoutCancel(ctx), cfgEnv.ShutdownTimeout)
	defer shutdownCancel()
	return errors.Join(serveErr, shutdown(shutdownCtx, logger, containerApp.Server, containerApp.Close))
}

func shutdown(ctx context.Context, logger *slog.Logger, server *http.Server, closeResources func(context.Context) error) error {
	logger.InfoContext(ctx, "service stopping")
	done := make(chan error, 1)
	go func() {
		httpErr := server.Shutdown(ctx)
		if httpErr != nil {
			httpErr = errors.Join(httpErr, server.Close())
		}
		done <- errors.Join(httpErr, closeResources(ctx))
	}()

	select {
	case err := <-done:
		if ctx.Err() != nil {
			logger.ErrorContext(ctx, "shutdown budget exhausted; forcing exit", "error", ctx.Err())
			return errors.Join(err, ctx.Err())
		}
		if err != nil {
			return fmt.Errorf("shutdown service: %w", err)
		}
		logger.InfoContext(ctx, "service stopped")
		return nil
	case <-ctx.Done():
		logger.ErrorContext(ctx, "shutdown budget exhausted; forcing exit", "error", ctx.Err())
		return fmt.Errorf("shutdown service: %w", ctx.Err())
	}
}
