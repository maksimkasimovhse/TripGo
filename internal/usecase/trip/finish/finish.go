package finish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type Usecase struct {
	logger      *slog.Logger
	txm         txManager
	tripRepo    tripRepo
	historyRepo historyRepo
	completed   metric.Int64Counter
}

func New(logger *slog.Logger, txm txManager, tripRepo tripRepo, historyRepo historyRepo, completed metric.Int64Counter) Usecase {
	return Usecase{logger: logger, txm: txm, tripRepo: tripRepo, historyRepo: historyRepo, completed: completed}
}

func (u *Usecase) Finish(ctx context.Context, tripID uuid.UUID) (trip.Trip, error) {
	var tripForFinish trip.Trip
	err := u.txm.Do(ctx, func(ctx context.Context) error {
		t, err := u.tripRepo.GetByID(ctx, tripID)
		if err != nil {
			return err
		}
		from := t.Status
		finishedAt := time.Now().UTC()
		if err := t.Finish(finishedAt); err != nil {
			return err
		}

		tripForFinish, err = u.tripRepo.Finish(ctx, tripID, *t.FinishedAt)
		if err != nil {
			return err
		}
		err = u.historyRepo.Add(ctx, tripID, &from, tripForFinish.Status, *tripForFinish.FinishedAt)
		if err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}

	u.completed.Add(ctx, 1)
	u.logger.InfoContext(ctx, "trip completed", "trip_id", tripForFinish.ID.String())
	return tripForFinish, nil
}
