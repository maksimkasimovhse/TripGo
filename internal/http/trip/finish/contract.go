package finish

import (
	"context"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

//go:generate go tool mockgen -source=$GOFILE -destination=mocks_test.go -package=finish_test -mock_names=finishUsecase=MockFinishUsecase

type finishUsecase interface {
	Finish(ctx context.Context, id uuid.UUID) (trip.Trip, error)
}
