package txmanager

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Executor interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type txKey struct{}

func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	checkCtx := ctx.Value(txKey{})
	v, ok := checkCtx.(pgx.Tx)
	return v, ok
}

func GetExecutor(ctx context.Context, pool *pgxpool.Pool) Executor {
	v, ok := txFromContext(ctx)
	if !ok {
		return pool
	}
	return v
}

type Manager struct {
	pool            *pgxpool.Pool
	isoLevel        pgx.TxIsoLevel
	rollbackTimeout time.Duration
}

func New(pool *pgxpool.Pool, isoLevel pgx.TxIsoLevel, rollbackTimeout time.Duration) *Manager {
	return &Manager{pool: pool, isoLevel: isoLevel, rollbackTimeout: rollbackTimeout}
}

func (m *Manager) Do(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
	}

	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: m.isoLevel})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() {
		ctxRollback, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.rollbackTimeout)
		defer cancel()
		_ = tx.Rollback(ctxRollback)
	}()

	if err := fn(withTx(ctx, tx)); err != nil {
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
