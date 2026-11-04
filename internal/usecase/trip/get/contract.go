package get

import (
	"context"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type tripRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
