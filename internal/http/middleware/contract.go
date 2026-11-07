package middleware

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/idempotency"
)

var _ api.ServerInterface = (*WithIdempotency)(nil)

type idempotencyUsecase interface {
	Acquire(ctx context.Context, key uuid.UUID, hash [32]byte) (idempotency.AcquireResult, error)
	Complete(ctx context.Context, key uuid.UUID) error
	Release(ctx context.Context, key uuid.UUID) error
}

type WithIdempotency struct {
	api.ServerInterface
	usecase idempotencyUsecase
	getter  tripGetter
}

func NewWithIdempotency(h api.ServerInterface, usecase idempotencyUsecase, getter tripGetter) *WithIdempotency {
	return &WithIdempotency{ServerInterface: h, usecase: usecase, getter: getter}
}

type tripGetter interface {
	Get(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
