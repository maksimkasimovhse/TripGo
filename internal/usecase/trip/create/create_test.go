package create_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
	"go.uber.org/mock/gomock"
)

func TestCreateSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	repo := NewMockTripRepository(ctrl)
	history := NewMockHistoryRepository(ctrl)
	txm := NewMockTxManager(ctrl)

	ctx := context.Background()
	txCtx, cancel := context.WithCancel(ctx)
	t.Cleanup(cancel)
	input := validInput()

	txm.EXPECT().
		Do(ctx, gomock.Any()).
		DoAndReturn(func(_ context.Context, fn func(context.Context) error) error {
			return fn(txCtx)
		})

	var saved trip.Trip
	gomock.InOrder(
		repo.EXPECT().
			Create(txCtx, gomock.Any()).
			DoAndReturn(func(_ context.Context, ride trip.Trip) error {
				saved = ride
				return nil
			}),
		history.EXPECT().
			Add(txCtx, input.ID, gomock.Nil(), trip.StatusActive, gomock.Any()).
			DoAndReturn(func(_ context.Context, _ uuid.UUID, _ *trip.Status, _ trip.Status, at time.Time) error {
				if !at.Equal(saved.StartedAt) {
					t.Errorf("history time = %v, want trip start time %v", at, saved.StartedAt)
				}
				return nil
			}),
	)

	uc := create.New(txm, repo, history)
	got, err := uc.Create(ctx, input)
	if err != nil {
		t.Fatalf("Create() error = %v, want nil", err)
	}

	want := trip.Trip{
		ID:             input.ID,
		UserID:         input.UserID,
		DriverID:       input.DriverID,
		StartLatitude:  input.StartLatitude,
		StartLongitude: input.StartLongitude,
		EndLatitude:    input.EndLatitude,
		EndLongitude:   input.EndLongitude,
		Price:          input.Price,
		Status:         trip.StatusActive,
		StartedAt:      got.StartedAt,
	}
	if got != want {
		t.Errorf("Create() = %+v, want %+v", got, want)
	}
	if saved != got {
		t.Errorf("saved trip = %+v, returned trip = %+v", saved, got)
	}
	if got.StartedAt.IsZero() {
		t.Error("StartedAt is zero")
	}
	if got.StartedAt.Location() != time.UTC {
		t.Errorf("StartedAt location = %v, want UTC", got.StartedAt.Location())
	}
	if !got.StartedAt.Equal(got.StartedAt.Truncate(time.Microsecond)) {
		t.Errorf("StartedAt = %v, want microsecond precision", got.StartedAt)
	}
}

func TestCreateInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		change    func(*create.Input)
		wantField string
	}{
		{name: "missing user", change: func(i *create.Input) { i.UserID = uuid.Nil }, wantField: "user_id"},
		{name: "missing driver", change: func(i *create.Input) { i.DriverID = uuid.Nil }, wantField: "driver_id"},
		{name: "invalid start latitude", change: func(i *create.Input) { i.StartLatitude = 91 }, wantField: "start_point"},
		{name: "invalid start longitude", change: func(i *create.Input) { i.StartLongitude = 181 }, wantField: "start_point"},
		{name: "invalid end latitude", change: func(i *create.Input) { i.EndLatitude = -91 }, wantField: "end_point"},
		{name: "invalid end longitude", change: func(i *create.Input) { i.EndLongitude = -181 }, wantField: "end_point"},
		{name: "negative price", change: func(i *create.Input) { i.Price = -1 }, wantField: "price"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := NewMockTripRepository(ctrl)
			history := NewMockHistoryRepository(ctrl)
			txm := NewMockTxManager(ctrl)
			input := validInput()
			tt.change(&input)

			uc := create.New(txm, repo, history)
			got, err := uc.Create(context.Background(), input)

			var validationErr *trip.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Create() error = %v, want *trip.ValidationError", err)
			}
			if validationErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", validationErr.Field, tt.wantField)
			}
			if got != (trip.Trip{}) {
				t.Errorf("Create() = %+v, want empty trip", got)
			}
		})
	}
}

func TestCreateDependencyErrors(t *testing.T) {
	t.Parallel()

	repoErr := errors.New("insert trip failed")
	historyErr := errors.New("insert history failed")
	beginErr := errors.New("begin transaction failed")
	commitErr := errors.New("commit transaction failed")
	tests := []struct {
		name       string
		repoErr    error
		historyErr error
		txErr      error
		beginFails bool
		wantErr    error
	}{
		{name: "driver busy", repoErr: trip.ErrDriverBusy, wantErr: trip.ErrDriverBusy},
		{name: "repository failure", repoErr: repoErr, wantErr: repoErr},
		{name: "history failure", historyErr: historyErr, wantErr: historyErr},
		{name: "begin failure", txErr: beginErr, beginFails: true, wantErr: beginErr},
		{name: "commit failure", txErr: commitErr, wantErr: commitErr},
		{name: "canceled repository operation", repoErr: context.Canceled, wantErr: context.Canceled},
		{name: "repository timeout", repoErr: context.DeadlineExceeded, wantErr: context.DeadlineExceeded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := NewMockTripRepository(ctrl)
			history := NewMockHistoryRepository(ctrl)
			txm := NewMockTxManager(ctrl)
			ctx := context.Background()
			txCtx, cancel := context.WithCancel(ctx)
			t.Cleanup(cancel)
			input := validInput()

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

				insert := repo.EXPECT().Create(txCtx, gomock.Any()).Return(tt.repoErr)
				if tt.repoErr == nil {
					history.EXPECT().
						Add(txCtx, input.ID, gomock.Nil(), trip.StatusActive, gomock.Any()).
						Return(tt.historyErr).
						After(insert)
				}
			}

			uc := create.New(txm, repo, history)
			got, err := uc.Create(ctx, input)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("Create() error = %v, want %v", err, tt.wantErr)
			}
			if got != (trip.Trip{}) {
				t.Errorf("Create() = %+v, want empty trip on error", got)
			}
		})
	}
}

func validInput() create.Input {
	return create.Input{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55,
		StartLongitude: 37,
		EndLatitude:    56,
		EndLongitude:   38,
		Price:          100,
	}
}
