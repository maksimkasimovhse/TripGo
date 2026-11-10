package trip_test

import (
	"errors"
	"math"
	"testing"

	"github.com/google/uuid"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

const (
	startPointField = "start_point"
	endPointField   = "end_point"
)

func TestTripValidate(t *testing.T) {
	valid := trip.Trip{
		UserID:         uuid.New(),
		DriverID:       uuid.New(),
		StartLatitude:  55.75,
		StartLongitude: 37.61,
		EndLatitude:    59.93,
		EndLongitude:   30.31,
		Price:          100,
	}

	with := func(change func(*trip.Trip)) trip.Trip {
		ride := valid
		change(&ride)
		return ride
	}
	tests := []struct {
		name      string
		ride      trip.Trip
		wantField string
	}{
		{
			name: "valid trip",
			ride: valid,
		},
		{
			name: "zero user ID",
			ride: with(func(r *trip.Trip) {
				r.UserID = uuid.Nil
			}),
			wantField: "user_id",
		},
		{
			name: "zero driver ID",
			ride: with(func(r *trip.Trip) {
				r.DriverID = uuid.Nil
			}),
			wantField: "driver_id",
		},
		{
			name: "negative price",
			ride: with(func(r *trip.Trip) {
				r.Price = -1
			}),
			wantField: "price",
		},
		{
			name: "zero price is valid",
			ride: with(func(r *trip.Trip) {
				r.Price = 0
			}),
		},
		{
			name: "start latitude minimum is valid",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = -90
			}),
		},
		{
			name: "start latitude maximum is valid",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = 90
			}),
		},
		{
			name: "start latitude below minimum",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = -90.001
			}),
			wantField: startPointField,
		},
		{
			name: "start latitude above maximum",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = 90.001
			}),
			wantField: startPointField,
		},
		{
			name: "start longitude minimum is valid",
			ride: with(func(r *trip.Trip) {
				r.StartLongitude = -180
			}),
		},
		{
			name: "start longitude maximum is valid",
			ride: with(func(r *trip.Trip) {
				r.StartLongitude = 180
			}),
		},
		{
			name: "start longitude below minimum",
			ride: with(func(r *trip.Trip) {
				r.StartLongitude = -180.001
			}),
			wantField: startPointField,
		},
		{
			name: "start longitude above maximum",
			ride: with(func(r *trip.Trip) {
				r.StartLongitude = 180.001
			}),
			wantField: startPointField,
		},
		{
			name: "end latitude minimum is valid",
			ride: with(func(r *trip.Trip) {
				r.EndLatitude = -90
			}),
		},
		{
			name: "end latitude maximum is valid",
			ride: with(func(r *trip.Trip) {
				r.EndLatitude = 90
			}),
		},
		{
			name: "end latitude below minimum",
			ride: with(func(r *trip.Trip) {
				r.EndLatitude = -90.001
			}),
			wantField: endPointField,
		},
		{
			name: "end latitude above maximum",
			ride: with(func(r *trip.Trip) {
				r.EndLatitude = 90.001
			}),
			wantField: endPointField,
		},
		{
			name: "end longitude minimum is valid",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = -180
			}),
		},
		{
			name: "end longitude maximum is valid",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = 180
			}),
		},
		{
			name: "end longitude below minimum",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = -180.001
			}),
			wantField: endPointField,
		},
		{
			name: "end longitude above maximum",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = 180.001
			}),
			wantField: endPointField,
		},
		{
			name: "zero coordinates are valid",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = 0
				r.StartLongitude = 0
				r.EndLatitude = 0
				r.EndLongitude = 0
			}),
		},
		{
			name: "NaN start latitude",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = math.NaN()
			}),
			wantField: startPointField,
		},
		{
			name: "NaN start longitude",
			ride: with(func(r *trip.Trip) {
				r.StartLongitude = math.NaN()
			}),
			wantField: startPointField,
		},
		{
			name: "NaN end latitude",
			ride: with(func(r *trip.Trip) {
				r.EndLatitude = math.NaN()
			}),
			wantField: endPointField,
		},
		{
			name: "NaN end longitude",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = math.NaN()
			}),
			wantField: endPointField,
		},
		{
			name: "positive infinity",
			ride: with(func(r *trip.Trip) {
				r.StartLatitude = math.Inf(1)
			}),
			wantField: startPointField,
		},
		{
			name: "negative infinity",
			ride: with(func(r *trip.Trip) {
				r.EndLongitude = math.Inf(-1)
			}),
			wantField: endPointField,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.ride.Validate()

			if tt.wantField == "" {
				if err != nil {
					t.Errorf("Validate() error = %v, want nil", err)
				}
				return
			}

			if err == nil {
				t.Fatalf("Validate() returned nil, want error for %q", tt.wantField)
			}

			var validationErr *trip.ValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("Validate() error = %v, want *trip.ValidationError", err)
			}
			if validationErr.Field != tt.wantField {
				t.Errorf("Field = %q, want %q", validationErr, tt.wantField)
			}
		})
	}
}
