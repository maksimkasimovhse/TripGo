package create

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
	created     metric.Int64Counter
}

func New(logger *slog.Logger, txm txManager, tripRepo tripRepo, historyRepo historyRepo, created metric.Int64Counter) Usecase {
	return Usecase{logger: logger, txm: txm, tripRepo: tripRepo, historyRepo: historyRepo, created: created}
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

	if err := tripForCreate.Validate(); err != nil {
		return trip.Trip{}, err
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

	u.created.Add(ctx, 1)
	u.logger.InfoContext(ctx, "trip created", "trip_id", tripForCreate.ID.String())
	return tripForCreate, nil
}
