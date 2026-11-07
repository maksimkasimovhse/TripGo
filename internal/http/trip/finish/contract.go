package finish

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type finishUsecase interface {
	Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
