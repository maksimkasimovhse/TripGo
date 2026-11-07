package main

import (
	"log"

	"github.com/maksimkasimovhse/TripGo/internal/app"
)

func main() {
	if err := app.Run(); err != nil {
		log.Fatalf("service stopped: %v", err)
	}
}
