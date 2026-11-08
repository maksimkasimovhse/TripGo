package get_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/get"

	"go.uber.org/mock/gomock"
)

func TestGetSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)

	id := uuid.New()
	ctx := context.Background()

	want := trip.Trip{
		ID:       id,
		UserID:   uuid.New(),
		DriverID: uuid.New(),
		Status:   trip.StatusActive,
		Price:    100,
	}

	repo.EXPECT().
		GetByID(gomock.Eq(ctx), gomock.Eq(id)).
		Return(want, nil).
		Times(1)

	uc := get.New(repo)

	got, err := uc.Get(ctx, id)

	if err != nil {
		t.Fatalf("Get() returned unexpected error: %v", err)
	}

	if got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

func TestGetRepositoryError(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)

	id := uuid.New()
	ctx := context.Background()
	repoErr := errors.New("database unavailable")
	want := trip.Trip{}

	repo.EXPECT().
		GetByID(gomock.Eq(ctx), gomock.Eq(id)).
		Return(want, repoErr).
		Times(1)

	uc := get.New(repo)

	got, err := uc.Get(ctx, id)

	if !errors.Is(err, repoErr) {
		t.Fatalf("Get() error = %v, want %v", err, repoErr)
	}

	if got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

func TestGetNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)

	id := uuid.New()
	ctx := context.Background()

	want := trip.Trip{}

	repo.EXPECT().
		GetByID(gomock.Eq(ctx), gomock.Eq(id)).
		Return(want, trip.ErrTripNotFound).
		Times(1)

	uc := get.New(repo)

	got, err := uc.Get(ctx, id)

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("Get() error = %v, want %v", err, trip.ErrTripNotFound)
	}

	if got != want {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}
