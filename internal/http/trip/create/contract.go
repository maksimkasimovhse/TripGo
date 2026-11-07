package create

import (
	"context"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

type createUsecase interface {
	Create(ctx context.Context, input create.Input) (trip.Trip, error)
}
