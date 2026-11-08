package trip_test

import (
	"testing"

	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
)

func TestValidationErrorError(t *testing.T) {
	t.Parallel()

	err := &trip.ValidationError{
		Field:   "price",
		Message: "must be >= 0",
	}

	got := err.Error()
	want := "price: must be >= 0"
	if got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}
