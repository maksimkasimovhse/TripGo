package create

import (
	"context"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mocks_test.go -package=create_test -mock_names=createUsecase=MockCreateUsecase

type createUsecase interface {
	Create(ctx context.Context, input createuc.Input) (trip.Trip, error)
}
