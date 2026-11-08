package finish_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/finish"
	"go.uber.org/mock/gomock"
)

func TestFinishTripSuccess(t *testing.T) {
	t.Parallel()

	uc := NewMockFinishUsecase(gomock.NewController(t))

	startedAt := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
	finishedAt := startedAt.Add(time.Hour)
	ride := trip.Trip{
		ID:             uuid.New(),
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    59.93,
		EndLongitude:   30.31,
		Price:          100,
		Status:         trip.StatusCompleted,
		StartedAt:      startedAt,
		FinishedAt:     &finishedAt,
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/"+ride.ID.String()+"/finish", nil)
	rec := httptest.NewRecorder()

	uc.EXPECT().
		Finish(req.Context(), ride.ID).
		Return(ride, nil)

	finish.New(uc).FinishTrip(rec, req, ride.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type=%q", got)
	}

	var body api.Trip

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}

	want := api.Trip{
		Id:         ride.ID,
		UserId:     ride.UserID,
		DriverId:   ride.DriverID,
		StartPoint: api.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   api.Coordinates{Latitude: 59.93, Longitude: 30.31},
		Price:      100,
		Status:     api.TripStatus(trip.StatusCompleted),
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("response=%+v want=%+v", body, want)
	}
}

func TestFinishTripErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{
			name:   "not found",
			err:    trip.ErrTripNotFound,
			status: http.StatusNotFound,
			code:   "trip_not_found",
		},
		{
			name:   "completed",
			err:    trip.ErrTripCompleted,
			status: http.StatusConflict,
			code:   "trip_completed",
		},
		{
			name:   "wrapped completed",
			err:    fmt.Errorf("finish: %w", trip.ErrTripCompleted),
			status: http.StatusConflict,
			code:   "trip_completed",
		},
		{
			name:   "internal",
			err:    errors.New("password=private"),
			status: http.StatusInternalServerError,
			code:   "internal_error",
		},
		{
			name:   "wrapped internal",
			err:    fmt.Errorf("finish: %w", errors.New("password=private")),
			status: http.StatusInternalServerError,
			code:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewMockFinishUsecase(gomock.NewController(t))
			id := uuid.New()

			path := "/api/v1/trips/" + id.String() + "/finish"

			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()

			uc.EXPECT().Finish(req.Context(), id).Return(trip.Trip{}, tt.err)

			finish.New(uc).FinishTrip(rec, req, id)

			if rec.Code != tt.status {
				t.Fatalf("status=%d want=%d body=%s", rec.Code, tt.status, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type=%q", got)
			}

			var body api.Problem

			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Code != tt.code || body.Status != int32(tt.status) {
				t.Errorf("problem=%+v want=%d %s", body, tt.status, tt.code)
			}
			if body.Instance == nil || *body.Instance != path {
				t.Errorf("Instance=%v want=%q", body.Instance, path)
			}
			if strings.Contains(rec.Body.String(), "password=private") {
				t.Error("internal error leaked")
			}
		})
	}
}

func TestFinishTripInvalidID(t *testing.T) {
	t.Parallel()

	uc := NewMockFinishUsecase(gomock.NewController(t))

	server := triphttp.New(nil, finish.New(uc), nil, nil, time.Second)

	router := api.HandlerWithOptions(server, api.ChiServerOptions{ErrorHandlerFunc: triphttp.ParamErrorHandler})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/not-a-uuid/finish", nil)
	rec := httptest.NewRecorder()

	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type=%q", got)
	}

	var body api.Problem

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != "invalid_request" || body.Status != http.StatusBadRequest {
		t.Errorf("problem=%+v", body)
	}
}
