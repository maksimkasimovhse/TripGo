package main

import (
	"log"

	"github.com/maksimkasimovhse/TripGo/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatal("service stopped: %w", err)
	}
}
