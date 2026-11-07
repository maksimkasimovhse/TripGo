package get

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type getUsecase interface {
	Get(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}

type Handler struct {
	getUsecase   getUsecase
}

func New(get getUsecase, readyTimeout time.Duration) *Handler {
	return &Handler{getUsecase: get}
}
