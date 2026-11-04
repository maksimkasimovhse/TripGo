package trip

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

type Handler struct {
	createUsecase createUsecase
	getUsecase    getUsecase
	finishUsecase finishUsecase

	readiness    readinessChecker
	readyTimeout time.Duration
}

func New(Create createUsecase, get getUsecase, finish finishUsecase, readiness readinessChecker, readyTimeout time.Duration) *Handler {
	return &Handler{createUsecase: Create, getUsecase: get, finishUsecase: finish, readiness: readiness, readyTimeout: readyTimeout}
}

type readinessChecker interface {
	Ping(ctx context.Context) error
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
