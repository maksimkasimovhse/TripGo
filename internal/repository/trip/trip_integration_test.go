//go:build integration

package repository_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
)

func TestTripCreateAndGetIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	want := trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    59.93,
		EndLongitude:   30.31,
		Price:          1250,
		Status:         trip.StatusActive,
		StartedAt:      time.Date(2026, 11, 9, 10, 0, 0, 123456000, time.UTC),
	}

	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()

		if _, err := pool.Exec(cleanupCtx, "DELETE FROM trips WHERE id = $1", want.ID); err != nil {
			t.Errorf("clean up test trip: %v", err)
		}
	})

	if err := repo.Create(ctx, want); err != nil {
		t.Fatalf("Create(): %v; ensure migrations are applied with make migrate", err)
	}

	got, err := repo.GetByID(ctx, want.ID)
	if err != nil {
		t.Fatalf("GetByID(): %v", err)
	}

	if got.CreatedAt.IsZero() || got.UpdatedAt.IsZero() {
		t.Fatalf("database timestamps are empty: CreatedAt=%v UpdatedAt=%v", got.CreatedAt, got.UpdatedAt)
	}
	if !got.CreatedAt.Equal(got.UpdatedAt) {
		t.Errorf("new trip UpdatedAt=%v, want CreatedAt=%v", got.UpdatedAt, got.CreatedAt)
	}
	if got.StartedAt.Location() != time.UTC || got.CreatedAt.Location() != time.UTC || got.UpdatedAt.Location() != time.UTC {
		t.Error("repository timestamps must be normalized to UTC")
	}

	want.CreatedAt = got.CreatedAt
	want.UpdatedAt = got.UpdatedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("GetByID() = %+v, want %+v", got, want)
	}
}

func TestTripGetNotFoundIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := repo.GetByID(ctx, uuid.New())

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("GetByID() error = %v, want %v", err, trip.ErrTripNotFound)
	}
	if got != (trip.Trip{}) {
		t.Errorf("GetByID() = %+v, want empty trip", got)
	}
}

func TestTripCreateDriverBusyIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first := integrationTrip()
	second := integrationTrip()
	second.DriverID = first.DriverID
	cleanupTrips(t, pool, first.ID, second.ID)

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first trip: %v", err)
	}
	before, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("read first trip: %v", err)
	}

	err = repo.Create(ctx, second)

	if !errors.Is(err, trip.ErrDriverBusy) {
		t.Fatalf("Create() error = %v, want %v", err, trip.ErrDriverBusy)
	}
	if _, err := repo.GetByID(ctx, second.ID); !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("rejected trip lookup error = %v, want %v", err, trip.ErrTripNotFound)
	}

	after, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("read first trip after conflict: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("first trip changed: got %+v, want %+v", after, before)
	}
}

func TestTripFinishActiveIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	if err := repo.Create(ctx, ride); err != nil {
		t.Fatalf("create trip: %v", err)
	}
	before, err := repo.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read active trip: %v", err)
	}
	finishedAt := ride.StartedAt.Add(time.Hour)

	got, err := repo.Finish(ctx, ride.ID, finishedAt)

	if err != nil {
		t.Fatalf("Finish(): %v", err)
	}
	want := before
	want.Status = trip.StatusCompleted
	want.FinishedAt = &finishedAt
	want.UpdatedAt = finishedAt
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Finish() = %+v, want %+v", got, want)
	}

	stored, err := repo.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read completed trip: %v", err)
	}
	if !reflect.DeepEqual(stored, want) {
		t.Errorf("stored trip = %+v, want %+v", stored, want)
	}
}

func TestTripFinishCompletedIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	if err := repo.Create(ctx, ride); err != nil {
		t.Fatalf("create trip: %v", err)
	}
	finishedAt := ride.StartedAt.Add(time.Hour)
	before, err := repo.Finish(ctx, ride.ID, finishedAt)
	if err != nil {
		t.Fatalf("first Finish(): %v", err)
	}

	got, err := repo.Finish(ctx, ride.ID, finishedAt.Add(time.Hour))

	if !errors.Is(err, trip.ErrTripCompleted) {
		t.Fatalf("second Finish() error = %v, want %v", err, trip.ErrTripCompleted)
	}
	if got != (trip.Trip{}) {
		t.Errorf("second Finish() = %+v, want empty trip", got)
	}

	after, err := repo.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read completed trip: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("completed trip changed: got %+v, want %+v", after, before)
	}
}

func TestTripFinishNotFoundIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	got, err := repo.Finish(ctx, uuid.New(), time.Now().UTC())

	if !errors.Is(err, trip.ErrTripNotFound) {
		t.Fatalf("Finish() error = %v, want %v", err, trip.ErrTripNotFound)
	}
	if got != (trip.Trip{}) {
		t.Errorf("Finish() = %+v, want empty trip", got)
	}
}

func TestTripCreateAfterFinishIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewTripRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	first := integrationTrip()
	second := integrationTrip()
	second.DriverID = first.DriverID
	second.StartedAt = first.StartedAt.Add(2 * time.Hour)
	cleanupTrips(t, pool, first.ID, second.ID)

	if err := repo.Create(ctx, first); err != nil {
		t.Fatalf("create first trip: %v", err)
	}
	completed, err := repo.Finish(ctx, first.ID, first.StartedAt.Add(time.Hour))
	if err != nil {
		t.Fatalf("finish first trip: %v", err)
	}

	if err := repo.Create(ctx, second); err != nil {
		t.Fatalf("create trip for released driver: %v", err)
	}

	stored, err := repo.GetByID(ctx, second.ID)
	if err != nil {
		t.Fatalf("read second trip: %v", err)
	}
	if stored.ID != second.ID || stored.DriverID != first.DriverID || stored.Status != trip.StatusActive || stored.FinishedAt != nil {
		t.Errorf("unexpected second trip: %+v", stored)
	}

	firstAfter, err := repo.GetByID(ctx, first.ID)
	if err != nil {
		t.Fatalf("read first trip: %v", err)
	}
	if !reflect.DeepEqual(firstAfter, completed) {
		t.Errorf("first trip changed: got %+v, want %+v", firstAfter, completed)
	}

	var activeCount int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM trips WHERE driver_id = $1 AND status = 'active'", first.DriverID).Scan(&activeCount); err != nil {
		t.Fatalf("count active trips: %v", err)
	}
	if activeCount != 1 {
		t.Errorf("active trips = %d, want 1", activeCount)
	}
}

func integrationTrip() trip.Trip {
	return trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    59.93,
		EndLongitude:   30.31,
		Price:          1250,
		Status:         trip.StatusActive,
		StartedAt:      time.Date(2026, 11, 9, 10, 0, 0, 123456000, time.UTC),
	}
}

func cleanupTrips(t *testing.T, pool *pgxpool.Pool, ids ...uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, id := range ids {
			if _, err := pool.Exec(ctx, "DELETE FROM trips WHERE id = $1", id); err != nil {
				t.Errorf("clean up test trip %s: %v", id, err)
			}
		}
	})
}

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		env, err := godotenv.Read(filepath.Join("..", "..", "..", ".env"))
		if err != nil {
			t.Fatal("set DATABASE_URL or create .env with tripgoctl environment start")
		}
		databaseURL = env["DATABASE_URL"]
	}
	if databaseURL == "" {
		t.Fatal("DATABASE_URL is required for integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create integration pool: %v", err)
	}
	t.Cleanup(pool.Close)

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("connect to integration PostgreSQL: %v", err)
	}

	return pool
}
