package trip_test

import (
	"errors"
	"testing"
	"time"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

func TestTripFinishActive(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	ride := trip.Trip{
		Status: trip.StatusActive,
	}

	err := ride.Finish(at)
	if err != nil {
		t.Fatalf("Finish() returned unexpected error: %v", err)
	}

	if ride.Status != trip.StatusCompleted {
		t.Errorf("Status = %q, want %q", ride.Status, trip.ErrTripCompleted)
	}

	if ride.FinishedAt == nil {
		t.Fatalf("FinishedAt is nil")
	}

	if !ride.FinishedAt.Equal(at) {
		t.Errorf("FinishedAt = %q, want %q", *ride.FinishedAt, at)
	}
}

func TestTripFinishCompleted(t *testing.T) {
	at := time.Date(2027, 11, 8, 12, 0, 0, 0, time.UTC)

	finishedAt := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
	ride := trip.Trip{
		Status:     trip.StatusCompleted,
		FinishedAt: &finishedAt,
	}

	wantFinishedAt := finishedAt

	err := ride.Finish(at)
	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Errorf("Finish() error = %v, want %v", err, trip.ErrTripCompleted)
	}

	if ride.Status != trip.StatusCompleted {
		t.Errorf("Status = %q, want %q", ride.Status, trip.StatusCompleted)
	}

	if ride.FinishedAt == nil {
		t.Fatalf("FinishedAt is nil")
	}

	if !ride.FinishedAt.Equal(wantFinishedAt) {
		t.Errorf("FinishedAt = %q, want %q", ride.FinishedAt, wantFinishedAt)
	}
}
