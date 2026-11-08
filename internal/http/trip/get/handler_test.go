package get_test

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
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/get"
	gomock "go.uber.org/mock/gomock"
)

func TestGetSuccess(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := NewMockGetUsecase(ctrl)

	id := uuid.New()
	startedAt := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	ride := trip.Trip{
		ID:        id,
		Status:    trip.StatusActive,
		Price:     100,
		StartedAt: startedAt,
	}

	req := httptest.NewRequest(
		http.MethodGet,
		"/api/v1/trips/"+id.String(),
		nil,
	)

	rec := httptest.NewRecorder()

	uc.EXPECT().
		Get(req.Context(), id).
		Return(ride, nil).
		Times(1)

	h := get.New(uc)

	h.GetTrip(rec, req, id)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s",
			rec.Code, http.StatusOK, rec.Body.String())
	}

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}

	var body api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if body.Id != id {
		t.Errorf("Id = %s, want %s", body.Id, id)
	}

	if body.Status != api.TripStatus(trip.StatusActive) {
		t.Errorf("Status = %q, want active", body.Status)
	}

	if body.Price != 100 {
		t.Errorf("Price = %d, want 100", body.Price)
	}

	if !body.StartedAt.Equal(startedAt) {
		t.Errorf("StartedAt = %v, want %v", body.StartedAt, startedAt)
	}
	if body.FinishedAt != nil {
		t.Errorf("FinishedAt = %v, want nil for active trip", body.FinishedAt)
	}
}

func TestGetCompletedTrip(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := NewMockGetUsecase(ctrl)
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
		Price:          1250,
		Status:         trip.StatusCompleted,
		StartedAt:      startedAt,
		FinishedAt:     &finishedAt,
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/"+ride.ID.String(), nil)
	rec := httptest.NewRecorder()
	uc.EXPECT().Get(req.Context(), ride.ID).Return(ride, nil)

	h := get.New(uc)
	h.GetTrip(rec, req, ride.ID)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var body api.Trip
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	want := api.Trip{
		Id:         ride.ID,
		UserId:     ride.UserID,
		DriverId:   ride.DriverID,
		StartPoint: api.Coordinates{Latitude: 55.75, Longitude: 37.61},
		EndPoint:   api.Coordinates{Latitude: 59.93, Longitude: 30.31},
		Price:      1250,
		Status:     api.TripStatus(trip.StatusCompleted),
		StartedAt:  startedAt,
		FinishedAt: &finishedAt,
	}
	if !reflect.DeepEqual(body, want) {
		t.Errorf("response = %+v, want %+v", body, want)
	}
}

func TestGetTripErrors(t *testing.T) {
	t.Parallel()

	const privateDetail = "database connection failed: password=private"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "wrapped not found",
			err:        fmt.Errorf("get trip: %w", trip.ErrTripNotFound),
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "internal error",
			err:        errors.New(privateDetail),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
		{
			name:       "wrapped internal error",
			err:        fmt.Errorf("get trip: %w", errors.New(privateDetail)),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := NewMockGetUsecase(ctrl)
			id := uuid.New()
			path := "/api/v1/trips/" + id.String()
			req := httptest.NewRequest(http.MethodGet, path, nil)
			rec := httptest.NewRecorder()
			uc.EXPECT().Get(req.Context(), id).Return(trip.Trip{}, tt.err)

			h := get.New(uc)
			h.GetTrip(rec, req, id)

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %s", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Errorf("Content-Type = %q, want application/problem+json", got)
			}
			var body api.Problem
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode problem: %v", err)
			}
			if body.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Status != int32(tt.wantStatus) {
				t.Errorf("body.Status = %d, want %d", body.Status, tt.wantStatus)
			}
			if body.Detail == nil {
				t.Fatal("Detail is nil")
			}
			if *body.Detail != tt.wantDetail {
				t.Errorf("Detail = %q, want %q", *body.Detail, tt.wantDetail)
			}
			if body.Instance == nil || *body.Instance != path {
				t.Errorf("Instance = %v, want %q", body.Instance, path)
			}
			if strings.Contains(rec.Body.String(), "password=private") {
				t.Error("response exposes internal error information")
			}
		})
	}
}

func TestGetTripInvalidID(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	uc := NewMockGetUsecase(ctrl)
	h := get.New(uc)
	server := triphttp.New(nil, nil, h, nil, time.Second)
	router := api.HandlerWithOptions(server, api.ChiServerOptions{
		ErrorHandlerFunc: triphttp.ParamErrorHandler,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trips/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body = %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", got)
	}
	var body api.Problem
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode problem: %v", err)
	}
	if body.Code != "invalid_request" || body.Status != http.StatusBadRequest {
		t.Errorf("problem = %+v, want 400 invalid_request", body)
	}
}
