package create_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	triphttp "github.com/maksimkasimovhse/TripGo/internal/http/trip"
	"github.com/maksimkasimovhse/TripGo/internal/http/trip/create"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

const validBody = `{"user_id":"5cb72c04-7650-45c9-a79b-bcdba0631e0c","driver_id":"8860b315-ec86-42eb-a17c-7c163d721ff5","start_point":{"latitude":55.75,"longitude":37.61},"end_point":{"latitude":59.93,"longitude":30.31},"price":100}`

func TestCreateTripSuccess(t *testing.T) {
	t.Parallel()

	for _, preset := range []bool{false, true} {
		t.Run(fmt.Sprintf("id from context=%t", preset), func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			uc := NewMockCreateUsecase(ctrl)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))

			assignedID := uuid.New()

			if preset {
				req = req.WithContext(triphttp.WithTripID(req.Context(), assignedID))
			}
			rec := httptest.NewRecorder()

			startedAt := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

			var createdID uuid.UUID

			uc.EXPECT().
				Create(req.Context(), gomock.Any()).
				DoAndReturn(func(_ context.Context, input createuc.Input) (trip.Trip, error) {
					createdID = input.ID

					if createdID == uuid.Nil || (preset && createdID != assignedID) {
						t.Fatalf("input ID = %s", createdID)
					}

					want := createuc.Input{
						ID:             createdID,
						UserID:         uuid.MustParse("5cb72c04-7650-45c9-a79b-bcdba0631e0c"),
						DriverID:       uuid.MustParse("8860b315-ec86-42eb-a17c-7c163d721ff5"),
						StartLatitude:  55.75,
						StartLongitude: 37.61,
						EndLatitude:    59.93,
						EndLongitude:   30.31,
						Price:          100,
					}
					if input != want {
						t.Errorf("input = %+v, want %+v", input, want)
					}

					return trip.Trip{
						ID:             input.ID,
						UserID:         input.UserID,
						DriverID:       input.DriverID,
						StartLatitude:  input.StartLatitude,
						StartLongitude: input.StartLongitude,
						EndLatitude:    input.EndLatitude,
						EndLongitude:   input.EndLongitude,
						Price:          input.Price,
						Status:         trip.StatusActive,
						StartedAt:      startedAt,
					}, nil
				})

			h := create.New(slog.New(slog.DiscardHandler), uc)

			h.CreateTrip(rec, req, api.CreateTripParams{})

			if rec.Code != http.StatusCreated {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := rec.Header().Get("Content-Type"); got != "application/json" {
				t.Errorf("Content-Type=%q", got)
			}
			if got := rec.Header().Get("Location"); got != "/api/v1/trips/"+createdID.String() {
				t.Errorf("Location=%q", got)
			}

			var body api.Trip

			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}

			want := api.Trip{
				Id:         createdID,
				UserId:     uuid.MustParse("5cb72c04-7650-45c9-a79b-bcdba0631e0c"),
				DriverId:   uuid.MustParse("8860b315-ec86-42eb-a17c-7c163d721ff5"),
				StartPoint: api.Coordinates{Latitude: 55.75, Longitude: 37.61},
				EndPoint:   api.Coordinates{Latitude: 59.93, Longitude: 30.31},
				Price:      100,
				Status:     api.TripStatus(trip.StatusActive),
				StartedAt:  startedAt,
			}
			if !reflect.DeepEqual(body, want) {
				t.Errorf("response=%+v want=%+v", body, want)
			}
		})
	}
}

func TestCreateTripUsecaseErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
		code   string
	}{
		{
			name:   "driver busy",
			err:    trip.ErrDriverBusy,
			status: http.StatusConflict,
			code:   "driver_busy",
		},
		{
			name:   "wrapped driver busy",
			err:    fmt.Errorf("create: %w", trip.ErrDriverBusy),
			status: http.StatusConflict,
			code:   "driver_busy",
		},
		{
			name:   "domain validation",
			err:    &trip.ValidationError{Field: "price", Message: "must be >= 0"},
			status: http.StatusBadRequest,
			code:   "invalid_request",
		},
		{
			name:   "internal",
			err:    errors.New("password=private"),
			status: http.StatusInternalServerError,
			code:   "internal_error",
		},
		{
			name:   "wrapped internal",
			err:    fmt.Errorf("create: %w", errors.New("password=private")),
			status: http.StatusInternalServerError,
			code:   "internal_error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewMockCreateUsecase(gomock.NewController(t))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(validBody))
			rec := httptest.NewRecorder()

			uc.EXPECT().Create(req.Context(), gomock.Any()).Return(trip.Trip{}, tt.err)

			create.New(slog.New(slog.DiscardHandler), uc).CreateTrip(rec, req, api.CreateTripParams{})

			checkCreateProblem(t, rec, tt.status, tt.code)
		})
	}
}

type unreadableBody struct{}

func (unreadableBody) Read([]byte) (int, error) {
	return 0, errors.New("private read failure")
}

func (unreadableBody) Close() error {
	return nil
}

func TestCreateTripInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, body string
		unreadable bool
	}{
		{
			name: "empty body",
		},
		{
			name: "malformed JSON",
			body: `{`,
		},
		{
			name: "null body",
			body: `null`,
		},
		{
			name: "missing fields",
			body: `{}`,
		},
		{
			name: "unknown field",
			body: strings.Replace(validBody, `"price":100`, `"price":100,"unknown":1`, 1),
		},
		{
			name: "invalid UUID",
			body: strings.Replace(validBody, "5cb72c04-7650-45c9-a79b-bcdba0631e0c", "not-a-uuid", 1),
		},
		{
			name: "invalid price type",
			body: strings.Replace(validBody, `"price":100`, `"price":"text"`, 1),
		},
		{
			name: "invalid coordinate type",
			body: strings.Replace(validBody, `"latitude":55.75`, `"latitude":"text"`, 1),
		},
		{
			name: "null required field",
			body: strings.Replace(validBody, `"price":100`, `"price":null`, 1),
		},
		{
			name: "too large",
			body: strings.Repeat("x", triphttp.MaxBodyBytes+1),
		},
		{
			name:       "read failure",
			body:       validBody,
			unreadable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc := NewMockCreateUsecase(gomock.NewController(t))

			req := httptest.NewRequest(http.MethodPost, "/api/v1/trips", strings.NewReader(tt.body))

			if tt.unreadable {
				req.Body = unreadableBody{}
			}
			rec := httptest.NewRecorder()

			create.New(slog.New(slog.DiscardHandler), uc).CreateTrip(rec, req, api.CreateTripParams{})

			checkCreateProblem(t, rec, http.StatusBadRequest, "invalid_request")

			if strings.Contains(rec.Body.String(), "private read failure") {
				t.Error("read error leaked")
			}
		})
	}
}

func checkCreateProblem(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()

	if rec.Code != status {
		t.Fatalf("status=%d want=%d body=%s", rec.Code, status, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Errorf("Content-Type=%q", got)
	}
	if got := rec.Header().Get("Location"); got != "" {
		t.Errorf("unexpected Location=%q", got)
	}

	var body api.Problem

	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != code || int(body.Status) != status {
		t.Errorf("problem=%+v want=%d %s", body, status, code)
	}
	if body.Instance == nil || *body.Instance != "/api/v1/trips" {
		t.Errorf("Instance=%v", body.Instance)
	}
	if strings.Contains(rec.Body.String(), "password=private") {
		t.Error("internal error leaked")
	}
}
