package finish_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/noop"
	"go.uber.org/mock/gomock"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/finish"
)

func TestFinishSuccess(t *testing.T) {
	t.Parallel()

	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	counter := &recordingCounter{}

	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	history := NewMockHistoryRepository(ctrl)
	txm := NewMockTxManager(ctrl)
	ctx := context.Background()
	txCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	active := activeTrip()

	txm.EXPECT().Do(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
			if err := fn(txCtx); err != nil {
				return err
			}
			if output.Len() != 0 {
				t.Error("business event logged before transaction commit")
			}
			if counter.value != 0 {
				t.Error("trip counted before transaction commit")
			}
			return nil
		})

	var saved trip.Trip
	from := trip.StatusActive
	gomock.InOrder(
		repo.EXPECT().GetByID(txCtx, active.ID).Return(active, nil),
		repo.EXPECT().Finish(txCtx, active.ID, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, at time.Time) (trip.Trip, error) {
				if at.IsZero() {
					t.Error("finish time is zero")
				}
				if at.Location() != time.UTC {
					t.Errorf("finish time location = %v, want UTC", at.Location())
				}
				persistedAt := at.Truncate(time.Microsecond)
				saved = active
				saved.Status = trip.StatusCompleted
				saved.FinishedAt = &persistedAt
				saved.UpdatedAt = persistedAt
				return saved, nil
			}),
		history.EXPECT().
			Add(txCtx, active.ID, gomock.Eq(&from), trip.StatusCompleted, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, _ *trip.Status, _ trip.Status, at time.Time) error {
				if !at.Equal(*saved.FinishedAt) {
					t.Errorf("history time = %v, want persisted finish time %v", at, *saved.FinishedAt)
				}
				return nil
			}),
	)

	uc := finish.New(logger, txm, repo, history, counter)
	got, err := uc.Finish(ctx, active.ID)
	if err != nil {
		t.Fatalf("Finish() error = %v, want nil", err)
	}
	if counter.value != 1 {
		t.Errorf("completed trips = %d, want 1", counter.value)
	}
	if !reflect.DeepEqual(got, saved) {
		t.Errorf("Finish() = %+v, want saved trip %+v", got, saved)
	}
	if got.Status != trip.StatusCompleted {
		t.Errorf("Status = %q, want %q", got.Status, trip.StatusCompleted)
	}
	if got.FinishedAt == nil {
		t.Fatal("FinishedAt is nil")
	}
	var record map[string]any
	if err := json.Unmarshal(output.Bytes(), &record); err != nil {
		t.Fatalf("expected one JSON business event: %v", err)
	}
	if record["msg"] != "trip completed" || record["trip_id"] != active.ID.String() || record["level"] != "INFO" {
		t.Errorf("business event = %+v", record)
	}
}

func TestFinishErrors(t *testing.T) {
	t.Parallel()

	getErr := errors.New("get trip failed")
	updateErr := errors.New("update trip failed")
	historyErr := errors.New("insert history failed")
	beginErr := errors.New("begin transaction failed")
	commitErr := errors.New("commit transaction failed")
	tests := []struct {
		name       string
		getErr     error
		finishErr  error
		historyErr error
		txErr      error
		completed  bool
		beginFails bool
		wantErr    error
	}{
		{name: "trip not found", getErr: trip.ErrTripNotFound, wantErr: trip.ErrTripNotFound},
		{name: "read failure", getErr: getErr, wantErr: getErr},
		{name: "already completed", completed: true, wantErr: trip.ErrTripCompleted},
		{name: "update failure", finishErr: updateErr, wantErr: updateErr},
		{name: "completed between read and update", finishErr: trip.ErrTripCompleted, wantErr: trip.ErrTripCompleted},
		{name: "deleted between read and update", finishErr: trip.ErrTripNotFound, wantErr: trip.ErrTripNotFound},
		{name: "history failure", historyErr: historyErr, wantErr: historyErr},
		{name: "begin failure", txErr: beginErr, beginFails: true, wantErr: beginErr},
		{name: "commit failure", txErr: commitErr, wantErr: commitErr},
		{name: "canceled read", getErr: context.Canceled, wantErr: context.Canceled},
		{name: "read timeout", getErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var output bytes.Buffer
			logger := slog.New(slog.NewJSONHandler(&output, nil))
			counter := &recordingCounter{}

			ctrl := gomock.NewController(t)
			repo := NewMockTripRepository(ctrl)
			history := NewMockHistoryRepository(ctrl)
			txm := NewMockTxManager(ctrl)
			ctx := context.Background()
			txCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			loaded := activeTrip()
			at := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)
			if tt.completed {
				loaded.Status = trip.StatusCompleted
				loaded.FinishedAt = &at
			}

			if tt.beginFails {
				txm.EXPECT().Do(ctx, gomock.Any()).Return(tt.txErr)
			} else {
				txm.EXPECT().Do(ctx, gomock.Any()).
					DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
						if err := fn(txCtx); err != nil {
							return err
						}
						return tt.txErr
					})

				read := repo.EXPECT().GetByID(txCtx, loaded.ID).Return(loaded, tt.getErr)
				if tt.getErr == nil && !tt.completed {
					persisted := loaded
					persisted.Status = trip.StatusCompleted
					persisted.FinishedAt = &at
					persisted.UpdatedAt = at
					update := repo.EXPECT().Finish(txCtx, loaded.ID, gomock.Any()).
						Return(persisted, tt.finishErr).After(read)
					if tt.finishErr == nil {
						from := trip.StatusActive
						history.EXPECT().
							Add(txCtx, loaded.ID, gomock.Eq(&from), trip.StatusCompleted, at).
							Return(tt.historyErr).After(update)
					}
				}
			}

			uc := finish.New(logger, txm, repo, history, counter)
			got, err := uc.Finish(ctx, loaded.ID)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Finish() error = %v, want %v", err, tt.wantErr)
			}
			if output.Len() != 0 {
				t.Errorf("business event logged for failed operation: %s", output.String())
			}
			if counter.value != 0 {
				t.Errorf("failed finish increased counter to %d", counter.value)
			}
			if got != (trip.Trip{}) {
				t.Errorf("Finish() = %+v, want empty trip on error", got)
			}
		})
	}
}

func activeTrip() trip.Trip {
	return trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          100,
		Status:         trip.StatusActive,
		StartedAt:      time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC),
	}
}

type recordingCounter struct {
	noop.Int64Counter
	value int64
}

func (c *recordingCounter) Add(_ context.Context, value int64, _ ...metric.AddOption) {
	c.value += value
}
