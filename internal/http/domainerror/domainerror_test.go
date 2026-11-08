package domainerror_test

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/domainerror"
)

func TestProblem(t *testing.T) {
	t.Parallel()

	const internalDetail = "database connection failed: password=secret"
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantCode   string
		wantDetail string
	}{
		{
			name:       "validation error",
			err:        &trip.ValidationError{Field: "price", Message: "must be >= 0"},
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "price: must be >= 0",
		},
		{
			name:       "trip not found",
			err:        trip.ErrTripNotFound,
			wantStatus: http.StatusNotFound,
			wantCode:   "trip_not_found",
			wantDetail: "Trip was not found",
		},
		{
			name:       "driver busy",
			err:        trip.ErrDriverBusy,
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name:       "trip completed",
			err:        trip.ErrTripCompleted,
			wantStatus: http.StatusConflict,
			wantCode:   "trip_completed",
			wantDetail: "Operation is not allowed for a completed trip",
		},
		{
			name:       "idempotency conflict",
			err:        idempotency.ErrIdempotencyKeyConflict,
			wantStatus: http.StatusConflict,
			wantCode:   "idempotency_conflict",
			wantDetail: "Idempotency-Key was already used with a different request body",
		},
		{
			name:       "request in progress",
			err:        idempotency.ErrRequestInProgress,
			wantStatus: http.StatusConflict,
			wantCode:   "request_in_progress",
			wantDetail: "Original request is still processing",
		},
		{
			name:       "wrapped sentinel error",
			err:        fmt.Errorf("create trip: %w", trip.ErrDriverBusy),
			wantStatus: http.StatusConflict,
			wantCode:   "driver_busy",
			wantDetail: "Driver already has an active trip",
		},
		{
			name: "wrapped validation error",
			err: fmt.Errorf("validate trip: %w", &trip.ValidationError{
				Field: "user_id", Message: "is required",
			}),
			wantStatus: http.StatusBadRequest,
			wantCode:   "invalid_request",
			wantDetail: "user_id: is required",
		},
		{
			name:       "internal error hides details",
			err:        errors.New(internalDetail),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
		{
			name:       "wrapped internal error hides details",
			err:        fmt.Errorf("get trip: %w", errors.New(internalDetail)),
			wantStatus: http.StatusInternalServerError,
			wantCode:   "internal_error",
			wantDetail: "Internal server error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, body := domainerror.Problem(tt.err)

			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if body.Status != int32(status) {
				t.Errorf("body.Status = %d, want returned status %d", body.Status, status)
			}
			if body.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", body.Code, tt.wantCode)
			}
			if body.Detail == nil {
				t.Fatal("Detail is nil")
			}
			if *body.Detail != tt.wantDetail {
				t.Errorf("Detail = %q, want %q", *body.Detail, tt.wantDetail)
			}
			if tt.wantStatus == http.StatusInternalServerError && strings.Contains(*body.Detail, "password=secret") {
				t.Error("Detail exposes internal error information")
			}
		})
	}
}
