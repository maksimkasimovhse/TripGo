package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os/signal"
	"syscall"

	"github.com/maksimkasimovhse/TripGo/internal/config"
	"github.com/maksimkasimovhse/TripGo/internal/container"
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

	containerApp, err := container.New(ctx, &cfgEnv)
	if err != nil {
		return fmt.Errorf("build application: %w", err)
	}
	defer containerApp.Close()

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

	if err = containerApp.Server.Shutdown(shutdownCtx); err != nil {
		_ = containerApp.Server.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}

	return nil
}
