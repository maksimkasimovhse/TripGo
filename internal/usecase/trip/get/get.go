package get

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type Usecase struct {
	tripRepo tripRepo
}

func New(tripRepo tripRepo) Usecase {
	return Usecase{tripRepo: tripRepo}
}

func (u *Usecase) Get(ctx context.Context, tripID uuid.UUID) (trip.Trip, error) {
	return u.tripRepo.GetByID(ctx, tripID)
}
