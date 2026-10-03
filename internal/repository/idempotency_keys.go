package repository

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
)

type IdempotencyStatus string

const (
	IdempotencyStatusProcessing IdempotencyStatus = "processing"
	IdempotencyStatusCompleted  IdempotencyStatus = "completed"
)

var ErrKeyNotFound = errors.New("idempotency key not found")

type IdempotencyKeyRepository struct {
	pool         *pgxpool.Pool
	sb           sq.StatementBuilderType
	queryTimeout time.Duration
}

func NewIdempotencyKeyRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *IdempotencyKeyRepository {
	return &IdempotencyKeyRepository{
		pool:         pool,
		sb:           sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		queryTimeout: queryTimeout,
	}
}

func (r *IdempotencyKeyRepository) TryInsert(ctx context.Context, id uuid.UUID, requestHash [32]byte, tripID uuid.UUID) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Insert("idempotency_keys").
		SetMap(map[string]any{
			"id":           id,
			"request_hash": requestHash[:],
			"trip_id":      tripID,
			"created_at":   time.Now(),
		}).
		Suffix("ON CONFLICT (id) DO NOTHING").
		ToSql()

	if err != nil {
		return false, fmt.Errorf("build insert idempotency key query: %w", err)
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

func (r *IdempotencyKeyRepository) Get(ctx context.Context, key uuid.UUID) (uuid.UUID, [32]byte, IdempotencyStatus, error) {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Select("trip_id", "request_hash", "status").
		From("idempotency_keys").
		Where(sq.Eq{"id": key}).
		ToSql()
	if err != nil {
		return uuid.Nil, [32]byte{}, "", fmt.Errorf("build select idempotency key query: %w", err)
	}

	var tripID uuid.UUID
	var hashBytes []byte
	var status IdempotencyStatus
	row := txmanager.GetExecutor(ctx, r.pool).QueryRow(ctx, query, args...)
	err = row.Scan(&tripID, &hashBytes, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, [32]byte{}, "", ErrKeyNotFound
	}
	if err != nil {
		return uuid.Nil, [32]byte{}, "", fmt.Errorf("scan idempotency key: %w", err)
	}
	if len(hashBytes) != sha256.Size {
		return uuid.Nil, [32]byte{}, "", fmt.Errorf("idempotency key %s: unexpected hash length %d", key, len(hashBytes))
	}

	var hash [32]byte
	copy(hash[:], hashBytes)
	return tripID, hash, status, nil
}

func (r *IdempotencyKeyRepository) Delete(ctx context.Context, key uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Delete("idempotency_keys").
		Where(sq.Eq{"id": key}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build delete idempotency key query: %w", err)
	}
	_, err = txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("delete recording: %w", err)
	}
	return nil
}

func (r *IdempotencyKeyRepository) MarkCompleted(ctx context.Context, key uuid.UUID) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()
	query, args, err := r.sb.
		Update("idempotency_keys").
		Set("status", string(IdempotencyStatusCompleted)).
		Where(sq.Eq{"id": key}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build update idempotency key status query: %w", err)
	}
	tag, err := txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("update status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("mark completed: idempotency key %s not found", key)
	}
	return nil
}
