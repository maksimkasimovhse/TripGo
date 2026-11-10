package idempotency

import (
	"context"
	"errors"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	"github.com/maksimkasimovhse/TripGo/internal/model"
)

type Usecase struct {
	idempotencyKeyRepo idempotencyKeyRepo
}

func New(idempotencyKeyRepo idempotencyKeyRepo) Usecase {
	return Usecase{idempotencyKeyRepo: idempotencyKeyRepo}
}

type AcquireResult struct {
	TripID uuid.UUID
	Replay bool
}

func (u *Usecase) Acquire(ctx context.Context, key uuid.UUID, hash [32]byte) (AcquireResult, error) {
	tripID := uuid.New()
	for attempt := 0; attempt < 2; attempt++ {
		inserted, err := u.idempotencyKeyRepo.TryInsert(ctx, key, hash, tripID)
		if err != nil {
			return AcquireResult{}, err
		}
		if inserted {
			return AcquireResult{TripID: tripID}, nil
		}

		savedTripID, savedHash, status, err := u.idempotencyKeyRepo.Get(ctx, key)
		if errors.Is(err, idempotency.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			return AcquireResult{}, err
		}
		if savedHash != hash {
			return AcquireResult{}, idempotency.ErrIdempotencyKeyConflict
		}

		switch status {
		case model.IdempotencyStatusProcessing:
			return AcquireResult{}, idempotency.ErrRequestInProgress

		case model.IdempotencyStatusCompleted:
			return AcquireResult{TripID: savedTripID, Replay: true}, nil

		default:
			return AcquireResult{}, errors.New("unexpected idempotency status")
		}
	}

	return AcquireResult{}, errors.New("failed to acquire idempotency key")
}

func (u *Usecase) Complete(ctx context.Context, key uuid.UUID) error {
	return u.idempotencyKeyRepo.MarkCompleted(ctx, key)
}

func (u *Usecase) Release(ctx context.Context, key uuid.UUID) error {
	return u.idempotencyKeyRepo.Delete(ctx, key)
}
