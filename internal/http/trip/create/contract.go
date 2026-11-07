package create

import (
	"context"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

type createUsecase interface {
	Create(ctx context.Context, input createuc.Input) (trip.Trip, error)
}
