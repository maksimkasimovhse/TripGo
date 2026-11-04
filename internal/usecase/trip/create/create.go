package create

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type Usecase struct {
	txm         txManager
	tripRepo    tripRepo
	historyRepo historyRepo
}

func New(txm txManager, tripRepo tripRepo, historyRepo historyRepo) Usecase {
	return Usecase{txm: txm, tripRepo: tripRepo, historyRepo: historyRepo}
}

type Input struct {
	ID             uuid.UUID
	UserID         uuid.UUID
	DriverID       uuid.UUID
	StartLatitude  float64
	StartLongitude float64
	EndLatitude    float64
	EndLongitude   float64
	Price          int64
}

func (u *Usecase) Create(ctx context.Context, input Input) (trip.Trip, error) {
	tripForCreate := trip.Trip{
		ID:             input.ID,
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         trip.StatusActive,
		StartedAt:      time.Now().UTC().Truncate(time.Microsecond),
	}

	err := u.txm.Do(ctx, func(ctx context.Context) error {
		if err := u.tripRepo.Create(ctx, tripForCreate); err != nil {
			return err
		}
		return u.historyRepo.Add(ctx, tripForCreate.ID, nil, trip.StatusActive, tripForCreate.StartedAt)
	})

	if err != nil {
		return trip.Trip{}, fmt.Errorf("create trip: %w", err)
	}

	return tripForCreate, nil
}
