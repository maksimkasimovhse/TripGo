//go:build integration

package repository_test

import (
	"context"
	"errors"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/infra/postgres/transaction"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	finishuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestTripLifecycleHistoryCommitIntegration(t *testing.T) {
	pool := integrationPool(t)
	trips := repository.NewTripRepository(pool, 3*time.Second)
	history := repository.NewStatusHistoryRepository(pool, 3*time.Second)
	txm := transaction.New(pool, pgx.ReadCommitted, 3*time.Second)
	creator := createuc.New(slog.New(slog.DiscardHandler), txm, trips, history, noop.Int64Counter{})
	finisher := finishuc.New(slog.New(slog.DiscardHandler), txm, trips, history, noop.Int64Counter{})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	input := createuc.Input{
		ID: ride.ID, UserID: ride.UserID, DriverID: ride.DriverID,
		StartLatitude: ride.StartLatitude, StartLongitude: ride.StartLongitude,
		EndLatitude: ride.EndLatitude, EndLongitude: ride.EndLongitude, Price: ride.Price,
	}

	created, err := creator.Create(ctx, input)
	if err != nil {
		t.Fatalf("create usecase: %v", err)
	}
	initialHistory := readHistory(t, ctx, pool, ride.ID)
	wantInitial := []historyEntry{{From: nil, To: string(trip.StatusActive), At: created.StartedAt}}
	if !reflect.DeepEqual(initialHistory, wantInitial) {
		t.Fatalf("initial history=%+v want=%+v", initialHistory, wantInitial)
	}

	completed, err := finisher.Finish(ctx, ride.ID)
	if err != nil {
		t.Fatalf("finish usecase: %v", err)
	}
	if completed.Status != trip.StatusCompleted || completed.FinishedAt == nil {
		t.Fatalf("completed trip=%+v", completed)
	}

	stored, err := trips.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read completed trip: %v", err)
	}
	if !reflect.DeepEqual(stored, completed) {
		t.Errorf("stored=%+v want=%+v", stored, completed)
	}

	active := string(trip.StatusActive)
	want := []historyEntry{
		{From: nil, To: string(trip.StatusActive), At: created.StartedAt},
		{From: &active, To: string(trip.StatusCompleted), At: *completed.FinishedAt},
	}
	if got := readHistory(t, ctx, pool, ride.ID); !reflect.DeepEqual(got, want) {
		t.Errorf("history=%+v want=%+v", got, want)
	}
}

func TestTripCreateHistoryRollbackIntegration(t *testing.T) {
	pool := integrationPool(t)
	trips := repository.NewTripRepository(pool, 3*time.Second)
	history := repository.NewStatusHistoryRepository(pool, 3*time.Second)
	txm := transaction.New(pool, pgx.ReadCommitted, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	historyAttempted := false

	err := txm.Do(ctx, func(txCtx context.Context) error {
		if err := trips.Create(txCtx, ride); err != nil {
			return err
		}
		historyAttempted = true
		return history.Add(txCtx, ride.ID, nil, trip.Status("invalid"), ride.StartedAt)
	})

	if !historyAttempted {
		t.Fatalf("trip INSERT failed before the history INSERT: %v", err)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("Do() error=%v, want CHECK violation 23514", err)
	}

	if _, err := trips.GetByID(ctx, ride.ID); !errors.Is(err, trip.ErrTripNotFound) {
		t.Errorf("trip survived rollback: lookup error=%v", err)
	}
	if got := readHistory(t, ctx, pool, ride.ID); len(got) != 0 {
		t.Errorf("history survived rollback: %+v", got)
	}
}

func TestTripFinishHistoryRollbackIntegration(t *testing.T) {
	pool := integrationPool(t)
	trips := repository.NewTripRepository(pool, 3*time.Second)
	history := repository.NewStatusHistoryRepository(pool, 3*time.Second)
	txm := transaction.New(pool, pgx.ReadCommitted, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	if err := trips.Create(ctx, ride); err != nil {
		t.Fatalf("create trip: %v", err)
	}
	before, err := trips.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read active trip: %v", err)
	}
	historyAttempted := false

	err = txm.Do(ctx, func(txCtx context.Context) error {
		completed, err := trips.Finish(txCtx, ride.ID, ride.StartedAt.Add(time.Hour))
		if err != nil {
			return err
		}
		historyAttempted = true
		from := trip.StatusActive
		return history.Add(txCtx, ride.ID, &from, trip.Status("invalid"), *completed.FinishedAt)
	})

	if !historyAttempted {
		t.Fatalf("UPDATE failed before history INSERT: %v", err)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" {
		t.Fatalf("Do() error=%v, want CHECK violation 23514", err)
	}

	after, err := trips.GetByID(ctx, ride.ID)
	if err != nil {
		t.Fatalf("read trip after rollback: %v", err)
	}
	if !reflect.DeepEqual(after, before) {
		t.Errorf("trip changed despite rollback: got=%+v want=%+v", after, before)
	}
	if got := readHistory(t, ctx, pool, ride.ID); len(got) != 0 {
		t.Errorf("history survived rollback: %+v", got)
	}
}
