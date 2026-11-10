package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/infra/postgres/transaction"
)

type TripRepository struct {
	pool         *pgxpool.Pool
	sb           sq.StatementBuilderType
	queryTimeout time.Duration
}

func NewTripRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *TripRepository {
	return &TripRepository{
		pool:         pool,
		sb:           sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		queryTimeout: queryTimeout,
	}
}

var tripColumns = []string{
	"id", "user_id", "driver_id",
	"start_latitude", "start_longitude", "end_latitude", "end_longitude",
	"price", "status",
	"started_at", "finished_at", "created_at", "updated_at",
}

func scanTrip(row pgx.Row) (trip.Trip, error) {
	var t trip.Trip
	err := row.Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartLatitude, &t.StartLongitude, &t.EndLatitude, &t.EndLongitude,
		&t.Price, &t.Status,
		&t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return t, err
	}

	t.StartedAt = t.StartedAt.UTC()
	t.CreatedAt = t.CreatedAt.UTC()
	t.UpdatedAt = t.UpdatedAt.UTC()
	if t.FinishedAt != nil {
		f := t.FinishedAt.UTC()
		t.FinishedAt = &f
	}
	return t, nil
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (trip.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Select(tripColumns...).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return trip.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	row := transaction.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	tripScan, err := scanTrip(row)

	if errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, trip.ErrTripNotFound
	}

	if err != nil {
		return trip.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return tripScan, nil
}

const activeTripPerDriverIdx = "trips_one_active_per_driver_idx"

func (r *TripRepository) CountActive(ctx context.Context) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	var count int64
	err := r.pool.QueryRow(ctx, "SELECT count(*) FROM trips WHERE status = 'active'").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active trips: %w", err)
	}
	return count, nil
}

func (r *TripRepository) Create(ctx context.Context, tripForCreate trip.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Insert("trips").
		SetMap(map[string]any{
			"id":              tripForCreate.ID,
			"user_id":         tripForCreate.UserID,
			"driver_id":       tripForCreate.DriverID,
			"start_latitude":  tripForCreate.StartLatitude,
			"start_longitude": tripForCreate.StartLongitude,
			"end_latitude":    tripForCreate.EndLatitude,
			"end_longitude":   tripForCreate.EndLongitude,
			"price":           tripForCreate.Price,
			"status":          tripForCreate.Status,
			"started_at":      tripForCreate.StartedAt,
		}).ToSql()

	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	_, err = transaction.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == activeTripPerDriverIdx {
			return trip.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}

	return nil

}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (trip.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Update("trips").
		SetMap(map[string]any{
			"status":      trip.StatusCompleted,
			"updated_at":  finishedAt,
			"finished_at": finishedAt,
		}).Where(sq.Eq{"id": id, "status": trip.StatusActive}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()

	if err != nil {
		return trip.Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}
	row := transaction.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	tripScan, err := scanTrip(row)

	if err == nil {
		return tripScan, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return trip.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	if _, err := r.GetByID(ctx, id); err != nil {
		return trip.Trip{}, err
	}
	return trip.Trip{}, trip.ErrTripCompleted
}
