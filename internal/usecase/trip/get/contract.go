package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mocks_test.go -package=get_test -mock_names=tripRepo=MockTripRepository

type tripRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
