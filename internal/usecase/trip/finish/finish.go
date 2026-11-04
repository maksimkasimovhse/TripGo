package finish

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

func (u *Usecase) Finish(ctx context.Context, tripId uuid.UUID) (trip.Trip, error) {
	var tripForFinish trip.Trip
	err := u.txm.Do(ctx, func(ctx context.Context) error {
		var err error
		finishedAt := time.Now().UTC()
		tripForFinish, err = u.tripRepo.Finish(ctx, tripId, finishedAt)
		if err != nil {
			return err
		}
		from := trip.StatusActive
		err = u.historyRepo.Add(ctx, tripId, &from, trip.StatusCompleted, finishedAt)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	return tripForFinish, nil
}
