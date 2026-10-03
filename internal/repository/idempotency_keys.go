package repository

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
)

type IdempocyKeyRepository struct {
	pool         *pgxpool.Pool
	sb           sq.StatementBuilderType
	queryTimeout time.Duration
}

func NewIdempocyKeyRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *IdempocyKeyRepository {
	return &IdempocyKeyRepository{
		pool:         pool,
		sb:           sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		queryTimeout: queryTimeout,
	}
}

func (r *IdempocyKeyRepository) TryInsert(ctx context.Context, id uuid.UUID, request_hash [32]byte, tripID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Insert("idempotency_keys").
		SetMap(map[string]any{
			"id":           id,
			"request_hash": request_hash,
			"trip_id":      tripID,
			"created_at":   time.Now(),
		}).
		Suffix("ON CONFLICT (id) DO NOTHING").
		ToSql()

	if err != nil {
		return false, fmt.Errorf("build insert status history query: %w", err)
	}

	tag, err := txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return false, fmt.Errorf("insert idempotency key: %w", err)
	}

	if tag.RowsAffected() == 1 {
		return true, nil
	}
	return false, nil
}

func (r *IdempocyKeyRepository) Get(ctx context.Context, id uuid.UUID) (uuid.UUID, [32]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Select("trip_id", "request_hash").
		From("idempotency_keys").
		Where(sq.Eq{"id": id}).
		ToSql()
	if err != nil {
		return uuid.Nil, [32]byte{}, fmt.Errorf("build insert status history query: %w", err)
	}

	var tripID uuid.UUID
	var hash [32]byte
	row := txmanager.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	if err = row.Scan(&tripID, &hash); err != nil {
		return uuid.Nil, [32]byte{}, fmt.Errorf("error assigning to a variable: %w", err)
	}

	return tripID, hash, nil

}

func (r *IdempocyKeyRepository) Delete(ctx context.Context, id uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.Delete("idempotency_keys").
	Where(sq.Eq{"id" : id}).
	ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}
	_, err = txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete recording: %w", err)
	}
	return nil
}
