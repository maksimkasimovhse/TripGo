package main

import (
	"log/slog"
	"os"

	"github.com/maksimkasimovhse/TripGo/internal/app"
	"github.com/maksimkasimovhse/TripGo/internal/infra/telemetry"
)

func main() {
	if err := app.Run(); err != nil {
		telemetry.NewLogger(slog.LevelInfo).Error("service failed", "error", err)
		os.Exit(1)
	}
}
