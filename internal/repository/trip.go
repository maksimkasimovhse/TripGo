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
	"github.com/maksimkasimovhse/TripGo/internal/domain"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
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

func scanTrip(row pgx.Row) (domain.Trip, error) {
	var t domain.Trip
	err := row.Scan(
		&t.ID, &t.UserID, &t.DriverID,
		&t.StartLatitude, &t.StartLongitude, &t.EndLatitude, &t.EndLongitude,
		&t.Price, &t.Status,
		&t.StartedAt, &t.FinishedAt, &t.CreatedAt, &t.UpdatedAt,
	)
	return t, err
}

func (r *TripRepository) GetByID(ctx context.Context, id uuid.UUID) (domain.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Select(tripColumns...).
		From("trips").
		Where(sq.Eq{"id": id}).
		ToSql()

	if err != nil {
		return domain.Trip{}, fmt.Errorf("build get trip query: %w", err)
	}

	row := txmanager.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	trip, err := scanTrip(row)

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, domain.ErrTripNotFound
	}

	if err != nil {
		return domain.Trip{}, fmt.Errorf("get trip: %w", err)
	}

	return trip, nil
}

const activeTripPerDriverIdx = "trips_one_active_per_driver_idx"

func (r *TripRepository) Create(ctx context.Context, trip domain.Trip) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Insert("trips").
		SetMap(map[string]any{
			"id":              trip.ID,
			"user_id":         trip.UserID,
			"driver_id":       trip.DriverID,
			"start_latitude":  trip.StartLatitude,
			"start_longitude": trip.StartLongitude,
			"end_latitude":    trip.EndLatitude,
			"end_longitude":   trip.EndLongitude,
			"price":           trip.Price,
			"status":          trip.Status,
			"started_at":      trip.StartedAt,
		}).ToSql()

	if err != nil {
		return fmt.Errorf("build create trip query: %w", err)
	}

	_, err = txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == activeTripPerDriverIdx {
			return domain.ErrDriverBusy
		}
		return fmt.Errorf("insert trip: %w", err)
	}

	return nil

}

func (r *TripRepository) Finish(ctx context.Context, id uuid.UUID, finishedAt time.Time) (domain.Trip, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Update("trips").
		SetMap(map[string]any{
			"status":      domain.StatusCompleted,
			"updated_at":  finishedAt,
			"finished_at": finishedAt,
		}).Where(sq.Eq{"id": id, "status": domain.StatusActive}).
		Suffix("RETURNING " + strings.Join(tripColumns, ", ")).
		ToSql()

	if err != nil {
		return domain.Trip{}, fmt.Errorf("build finish trip query: %w", err)
	}
	row := txmanager.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	trip, err := scanTrip(row)

	if err == nil {
		return trip, nil
	}

	if !errors.Is(err, pgx.ErrNoRows) {
		return domain.Trip{}, fmt.Errorf("finish trip: %w", err)
	}
	if _, err := r.GetByID(ctx, id); err != nil {
		return domain.Trip{}, err
	}
	return domain.Trip{}, domain.ErrTripCompleted
}
