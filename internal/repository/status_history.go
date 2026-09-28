package repository

import (
	"context"
	"fmt"
	"time"

	sq "github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/maksimkasimovhse/TripGo/internal/domain"
	"github.com/maksimkasimovhse/TripGo/internal/txmanager"
)

type StatusHistoryRepository struct {
	pool         *pgxpool.Pool
	sb           sq.StatementBuilderType
	queryTimeout time.Duration
}

func NewStatusHistoryRepository(pool *pgxpool.Pool, queryTimeout time.Duration) *StatusHistoryRepository {
	return &StatusHistoryRepository{
		pool:         pool,
		sb:           sq.StatementBuilder.PlaceholderFormat(sq.Dollar),
		queryTimeout: queryTimeout,
	}
}

func (r *StatusHistoryRepository) Add(ctx context.Context, tripID uuid.UUID, from *domain.Status, to domain.Status, changedAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, r.queryTimeout)
	defer cancel()

	query, args, err := r.sb.
		Insert("trip_status_history").
		SetMap(map[string]any{
			"trip_id":     tripID,
			"from_status": from,
			"to_status":   to,
			"changed_at":  changedAt,
		}).
		ToSql()
	if err != nil {
		return fmt.Errorf("build insert status history query: %w", err)
	}

	if _, err := txmanager.GetExecutor(ctx, r.pool).Exec(ctx, query, args...); err != nil {
		return fmt.Errorf("insert status history: %w", err)
	}
	return nil
}
