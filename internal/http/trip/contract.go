package trip

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

type Handler struct {
	createUsecase createUsecase
	getUsecase    getUsecase
	finishUsecase finishUsecase
}

func New(Create createUsecase, get getUsecase, finish finishUsecase) *Handler {
	return &Handler{createUsecase: Create, getUsecase: get, finishUsecase: finish}
}

type createUsecase interface {
	Create(ctx context.Context, input create.Input) (trip.Trip, error)
}

type getUsecase interface {
	Get(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}

type finishUsecase interface {
	Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
