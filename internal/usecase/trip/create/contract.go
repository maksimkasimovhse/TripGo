package create

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

type tripRepo interface {
	Create(ctx context.Context, trip trip.Trip) error
}

type historyRepo interface {
	Add(ctx context.Context, tripID uuid.UUID, from *trip.Status, to trip.Status, changedAt time.Time) error
}

type txManager interface {
	Do(ctx context.Context, fn func(ctx context.Context) error) error
}
