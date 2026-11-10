//go:build integration

package repository_test

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
)

type historyEntry struct {
	From *string
	To   string
	At   time.Time
}

func TestStatusHistoryAddIntegration(t *testing.T) {
	pool := integrationPool(t)
	trips := repository.NewTripRepository(pool, 3*time.Second)
	history := repository.NewStatusHistoryRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ride := integrationTrip()
	cleanupTrips(t, pool, ride.ID)
	if err := trips.Create(ctx, ride); err != nil {
		t.Fatalf("create trip: %v", err)
	}

	if err := history.Add(ctx, ride.ID, nil, trip.StatusActive, ride.StartedAt); err != nil {
		t.Fatalf("add initial history: %v", err)
	}
	from := trip.StatusActive
	finishedAt := ride.StartedAt.Add(time.Hour)
	if err := history.Add(ctx, ride.ID, &from, trip.StatusCompleted, finishedAt); err != nil {
		t.Fatalf("add completion history: %v", err)
	}

	got := readHistory(t, ctx, pool, ride.ID)
	active := string(trip.StatusActive)
	want := []historyEntry{
		{From: nil, To: string(trip.StatusActive), At: ride.StartedAt},
		{From: &active, To: string(trip.StatusCompleted), At: finishedAt},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("history=%+v want=%+v", got, want)
	}
}

func TestStatusHistoryMissingTripIntegration(t *testing.T) {
	pool := integrationPool(t)
	history := repository.NewStatusHistoryRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	id := uuid.New()

	err := history.Add(ctx, id, nil, trip.StatusActive, time.Now().UTC())

	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23503" {
		t.Fatalf("Add() error=%v, want foreign key violation 23503", err)
	}
	if got := readHistory(t, ctx, pool, id); len(got) != 0 {
		t.Errorf("orphan history=%+v", got)
	}
}

func readHistory(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) []historyEntry {
	t.Helper()

	rows, err := pool.Query(ctx, "SELECT from_status, to_status, changed_at FROM trip_status_history WHERE trip_id = $1 ORDER BY id", id)
	if err != nil {
		t.Fatalf("read history: %v", err)
	}
	defer rows.Close()

	var result []historyEntry
	for rows.Next() {
		var entry historyEntry
		if err := rows.Scan(&entry.From, &entry.To, &entry.At); err != nil {
			t.Fatalf("scan history: %v", err)
		}
		entry.At = entry.At.UTC()
		result = append(result, entry)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate history: %v", err)
	}

	return result
}
