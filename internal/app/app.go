package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/maksimkasimovhse/TripGo/internal/config"
	"github.com/maksimkasimovhse/TripGo/internal/container"
	"github.com/maksimkasimovhse/TripGo/internal/infra/telemetry"
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

	var level slog.Level
	if err := level.UnmarshalText([]byte(cfgEnv.LogLevel)); err != nil {
		return fmt.Errorf("parse log level: %w", err)
	}
	logger := telemetry.NewLogger(level)

	containerApp, err := container.New(ctx, logger, &cfgEnv)
	if err != nil {
		return fmt.Errorf("build application: %w", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), cfgEnv.ShutdownTimeout)
		defer cancel()
		if err := containerApp.Close(shutdownCtx); err != nil {
			logger.ErrorContext(shutdownCtx, "telemetry shutdown failed", "error", err)
		}
	}()
	logger.InfoContext(ctx, "service starting", "addr", cfgEnv.HTTPAddr)

	srvErr := make(chan error, 1)
	go func() {
		if err := containerApp.Server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
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
	logger.InfoContext(shutdownCtx, "service stopping")

	if err = containerApp.Server.Shutdown(shutdownCtx); err != nil {
		_ = containerApp.Server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	logger.InfoContext(shutdownCtx, "service stopped")
	return nil
}
