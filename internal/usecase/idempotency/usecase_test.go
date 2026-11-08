package idempotency_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	domain "github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	"github.com/maksimkasimovhse/TripGo/internal/model"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/idempotency"
	"go.uber.org/mock/gomock"
)

func TestAcquireNewKey(t *testing.T) {
	t.Parallel()

	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry=%t", retry), func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := NewMockIdempotencyKeyRepository(ctrl)
			ctx := context.Background()
			key := uuid.New()
			hash := sha256.Sum256([]byte("request"))
			var generatedID uuid.UUID

			first := repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).
				DoAndReturn(func(_ context.Context, _ uuid.UUID, _ [32]byte, id uuid.UUID) (bool, error) {
					generatedID = id
					if id == uuid.Nil {
						t.Error("generated trip ID is nil")
					}
					return !retry, nil
				})
			if retry {
				read := repo.EXPECT().Get(ctx, key).
					Return(uuid.Nil, [32]byte{}, model.IdempotencyStatus(""), fmt.Errorf("read key: %w", domain.ErrKeyNotFound)).
					After(first)
				repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).
					DoAndReturn(func(_ context.Context, _ uuid.UUID, _ [32]byte, id uuid.UUID) (bool, error) {
						if id != generatedID {
							t.Errorf("retry trip ID = %s, want original %s", id, generatedID)
						}
						return true, nil
					}).After(read)
			}

			uc := idempotency.New(repo)
			got, err := uc.Acquire(ctx, key, hash)
			if err != nil {
				t.Fatalf("Acquire() error = %v, want nil", err)
			}
			want := idempotency.AcquireResult{TripID: generatedID}
			if got != want {
				t.Errorf("Acquire() = %+v, want %+v", got, want)
			}
		})
	}
}

func TestAcquireExistingKey(t *testing.T) {
	t.Parallel()

	hash := sha256.Sum256([]byte("request"))
	differentHash := sha256.Sum256([]byte("different request"))
	readErr := errors.New("read key failed")
	tests := []struct {
		name        string
		savedHash   [32]byte
		status      model.IdempotencyStatus
		getErr      error
		wantErr     error
		wantMessage string
	}{
		{name: "completed replay", savedHash: hash, status: model.IdempotencyStatusCompleted},
		{name: "processing", savedHash: hash, status: model.IdempotencyStatusProcessing, wantErr: domain.ErrRequestInProgress},
		{name: "completed with different body", savedHash: differentHash, status: model.IdempotencyStatusCompleted, wantErr: domain.ErrIdempotencyKeyConflict},
		{name: "processing with different body", savedHash: differentHash, status: model.IdempotencyStatusProcessing, wantErr: domain.ErrIdempotencyKeyConflict},
		{name: "read failure", getErr: readErr, wantErr: readErr},
		{name: "unknown status", savedHash: hash, status: model.IdempotencyStatus("unknown"), wantMessage: "unexpected idempotency status"},
	}

	for _, tt := range tests {
		for _, retry := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/retry=%t", tt.name, retry), func(t *testing.T) {
				t.Parallel()

				ctrl := gomock.NewController(t)
				repo := NewMockIdempotencyKeyRepository(ctrl)
				ctx := context.Background()
				key, savedID := uuid.New(), uuid.New()
				var lastInsert *gomock.Call
				if retry {
					insert := repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, nil)
					missing := repo.EXPECT().Get(ctx, key).
						Return(uuid.Nil, [32]byte{}, model.IdempotencyStatus(""), domain.ErrKeyNotFound).After(insert)
					lastInsert = repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, nil).After(missing)
				} else {
					lastInsert = repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, nil)
				}
				repo.EXPECT().Get(ctx, key).Return(savedID, tt.savedHash, tt.status, tt.getErr).After(lastInsert)

				uc := idempotency.New(repo)
				got, err := uc.Acquire(ctx, key, hash)
				if tt.wantMessage != "" {
					if err == nil || err.Error() != tt.wantMessage {
						t.Fatalf("Acquire() error = %v, want %q", err, tt.wantMessage)
					}
				} else if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Acquire() error = %v, want %v", err, tt.wantErr)
				}

				want := idempotency.AcquireResult{}
				if tt.wantErr == nil && tt.wantMessage == "" {
					want = idempotency.AcquireResult{TripID: savedID, Replay: true}
				}
				if got != want {
					t.Errorf("Acquire() = %+v, want %+v", got, want)
				}
			})
		}
	}
}

func TestAcquireInsertError(t *testing.T) {
	t.Parallel()

	for _, retry := range []bool{false, true} {
		t.Run(fmt.Sprintf("retry=%t", retry), func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := NewMockIdempotencyKeyRepository(ctrl)
			ctx := context.Background()
			key := uuid.New()
			hash := sha256.Sum256([]byte("request"))
			insertErr := errors.New("insert key failed")
			if retry {
				insert := repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, nil)
				read := repo.EXPECT().Get(ctx, key).
					Return(uuid.Nil, [32]byte{}, model.IdempotencyStatus(""), domain.ErrKeyNotFound).After(insert)
				repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, insertErr).After(read)
			} else {
				repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, insertErr)
			}

			uc := idempotency.New(repo)
			got, err := uc.Acquire(ctx, key, hash)
			if !errors.Is(err, insertErr) {
				t.Fatalf("Acquire() error = %v, want %v", err, insertErr)
			}
			if got != (idempotency.AcquireResult{}) {
				t.Errorf("Acquire() = %+v, want empty result", got)
			}
		})
	}
}

func TestAcquireRetryExhausted(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := NewMockIdempotencyKeyRepository(ctrl)
	ctx := context.Background()
	key := uuid.New()
	hash := sha256.Sum256([]byte("request"))
	var previous *gomock.Call
	for attempt := 0; attempt < 2; attempt++ {
		insert := repo.EXPECT().TryInsert(ctx, key, hash, gomock.Any()).Return(false, nil)
		if previous != nil {
			insert.After(previous)
		}
		previous = repo.EXPECT().Get(ctx, key).
			Return(uuid.Nil, [32]byte{}, model.IdempotencyStatus(""), domain.ErrKeyNotFound).After(insert)
	}

	uc := idempotency.New(repo)
	got, err := uc.Acquire(ctx, key, hash)
	if err == nil || err.Error() != "failed to acquire idempotency key" {
		t.Fatalf("Acquire() error = %v, want retry exhaustion error", err)
	}
	if got != (idempotency.AcquireResult{}) {
		t.Errorf("Acquire() = %+v, want empty result", got)
	}
}

func TestCompleteAndRelease(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("update key failed")
	tests := []struct {
		name    string
		release bool
		wantErr error
	}{
		{name: "complete success"},
		{name: "complete failure", wantErr: repoErr},
		{name: "release success", release: true},
		{name: "release failure", release: true, wantErr: repoErr},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := NewMockIdempotencyKeyRepository(ctrl)
			ctx := context.Background()
			key := uuid.New()
			uc := idempotency.New(repo)
			var err error
			if tt.release {
				repo.EXPECT().Delete(ctx, key).Return(tt.wantErr)
				err = uc.Release(ctx, key)
			} else {
				repo.EXPECT().MarkCompleted(ctx, key).Return(tt.wantErr)
				err = uc.Complete(ctx, key)
			}
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}
