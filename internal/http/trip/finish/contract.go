package finish

import (
	"context"
	"time"
	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type finishUsecase interface {
	Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}

type Handler struct {
	finishUsecase finishUsecase
}

func New(finish finishUsecase, readyTimeout time.Duration) *Handler {
	return &Handler{finishUsecase: finish}
}
