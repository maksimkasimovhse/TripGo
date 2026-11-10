package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/maksimkasimovhse/TripGo/internal/app"
	"github.com/maksimkasimovhse/TripGo/internal/infra/telemetry"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	err := app.Run(ctx)
	stop()
	if err != nil {
		telemetry.NewLogger(slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}
