//go:build integration

package repository_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	"github.com/maksimkasimovhse/TripGo/internal/model"
	"github.com/maksimkasimovhse/TripGo/internal/repository/trip"
)

func TestIdempotencyInsertAndGetIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key, tripID := uuid.New(), uuid.New()
	hash := sha256.Sum256([]byte("request"))
	cleanupKeys(t, pool, key)

	inserted, err := repo.TryInsert(ctx, key, hash, tripID)
	if err != nil || !inserted {
		t.Fatalf("TryInsert()=%t,%v want true,nil", inserted, err)
	}

	gotID, gotHash, status, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if gotID != tripID || gotHash != hash || status != model.IdempotencyStatusProcessing {
		t.Errorf("Get()=%s,%x,%q want=%s,%x,processing", gotID, gotHash, status, tripID, hash)
	}
}

func TestIdempotencyDuplicateIntegration(t *testing.T) {
	for _, completed := range []bool{false, true} {
		name := "processing"
		if completed {
			name = "completed"
		}
		t.Run(name, func(t *testing.T) {
			pool := integrationPool(t)
			repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			key, tripID := uuid.New(), uuid.New()
			hash := sha256.Sum256([]byte("original request"))
			cleanupKeys(t, pool, key)
			inserted, err := repo.TryInsert(ctx, key, hash, tripID)
			if err != nil || !inserted {
				t.Fatalf("initial TryInsert()=%t,%v", inserted, err)
			}
			wantStatus := model.IdempotencyStatusProcessing
			if completed {
				if err := repo.MarkCompleted(ctx, key); err != nil {
					t.Fatalf("MarkCompleted(): %v", err)
				}
				wantStatus = model.IdempotencyStatusCompleted
			}

			inserted, err = repo.TryInsert(ctx, key, sha256.Sum256([]byte("different request")), uuid.New())
			if err != nil || inserted {
				t.Fatalf("duplicate TryInsert()=%t,%v want false,nil", inserted, err)
			}

			gotID, gotHash, status, err := repo.Get(ctx, key)
			if err != nil {
				t.Fatalf("Get(): %v", err)
			}
			if gotID != tripID || gotHash != hash || status != wantStatus {
				t.Errorf("duplicate overwrote saved key: %s,%x,%q", gotID, gotHash, status)
			}
		})
	}
}

func TestIdempotencyMarkCompletedIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key, tripID := uuid.New(), uuid.New()
	hash := sha256.Sum256([]byte("request"))
	cleanupKeys(t, pool, key)
	inserted, err := repo.TryInsert(ctx, key, hash, tripID)
	if err != nil || !inserted {
		t.Fatalf("TryInsert()=%t,%v", inserted, err)
	}

	if err := repo.MarkCompleted(ctx, key); err != nil {
		t.Fatalf("MarkCompleted(): %v", err)
	}

	gotID, gotHash, status, err := repo.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get(): %v", err)
	}
	if status != model.IdempotencyStatusCompleted || gotID != tripID || gotHash != hash {
		t.Errorf("completed key=%s,%x,%q", gotID, gotHash, status)
	}
}

func TestIdempotencyDeleteIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := uuid.New()
	cleanupKeys(t, pool, key)
	inserted, err := repo.TryInsert(ctx, key, sha256.Sum256([]byte("request")), uuid.New())
	if err != nil || !inserted {
		t.Fatalf("TryInsert()=%t,%v", inserted, err)
	}

	if err := repo.Delete(ctx, key); err != nil {
		t.Fatalf("Delete(): %v", err)
	}

	if _, _, _, err := repo.Get(ctx, key); !errors.Is(err, idempotency.ErrKeyNotFound) {
		t.Errorf("Get() error=%v want ErrKeyNotFound", err)
	}
	if err := repo.Delete(ctx, key); err != nil {
		t.Errorf("repeated Delete(): %v", err)
	}
}

func TestIdempotencyGetMissingIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	id, hash, status, err := repo.Get(ctx, uuid.New())

	if !errors.Is(err, idempotency.ErrKeyNotFound) {
		t.Fatalf("Get() error=%v want ErrKeyNotFound", err)
	}
	if id != uuid.Nil || hash != ([32]byte{}) || status != "" {
		t.Errorf("missing key result=%s,%x,%q", id, hash, status)
	}
}

func TestIdempotencyMarkMissingIntegration(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewIdempotencyKeyRepository(pool, 3*time.Second)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	key := uuid.New()
	cleanupKeys(t, pool, key)

	if err := repo.MarkCompleted(ctx, key); err == nil {
		t.Fatal("MarkCompleted() unexpectedly succeeded")
	}
	if _, _, _, err := repo.Get(ctx, key); !errors.Is(err, idempotency.ErrKeyNotFound) {
		t.Errorf("MarkCompleted created key: Get() error=%v", err)
	}
}

func cleanupKeys(t *testing.T, pool *pgxpool.Pool, ids ...uuid.UUID) {
	t.Helper()

	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		for _, id := range ids {
			if _, err := pool.Exec(ctx, "DELETE FROM idempotency_keys WHERE id = $1", id); err != nil {
				t.Errorf("clean up key %s: %v", id, err)
			}
		}
	})
}
