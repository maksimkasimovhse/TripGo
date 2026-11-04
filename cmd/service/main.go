package main

import (
	"log"

	"github.com/maksimkasimovhse/TripGo/internal/container"
)

func main() {
	if err := container.Run(); err != nil {
		log.Fatalf("service stopped: %v", err)
	}
}
