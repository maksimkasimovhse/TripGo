package idempotency

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/model"
)

type idempotencyKeyRepo interface {
	TryInsert(ctx context.Context, id uuid.UUID, requestHash [32]byte, tripID uuid.UUID) (bool, error)
	Get(ctx context.Context, key uuid.UUID) (uuid.UUID, [32]byte, model.IdempotencyStatus, error)
	Delete(ctx context.Context, key uuid.UUID) error
	MarkCompleted(ctx context.Context, key uuid.UUID) error
}
