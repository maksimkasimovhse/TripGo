package finish

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mocks_test.go -package=finish_test -mock_names=tripRepo=MockTripRepository,historyRepo=MockHistoryRepository,txManager=MockTxManager

type tripRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error)
	Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (trip.Trip, error)
}

type historyRepo interface {
	Add(ctx context.Context, tripID uuid.UUID, from *trip.Status, to trip.Status, changedAt time.Time) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
