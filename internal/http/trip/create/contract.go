package create

import (
	"context"
	"time"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

type createUsecase interface {
	Create(ctx context.Context, input create.Input) (trip.Trip, error)
}

type Handler struct {
	createUsecase createUsecase
}

func New(Create createUsecase, readyTimeout time.Duration) *Handler {
	return &Handler{createUsecase: Create}
}
