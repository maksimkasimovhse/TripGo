package trip

import (
	"github.com/google/uuid"
)

func (t Trip) Validate() error {
	switch {
	case t.UserID == uuid.Nil:
		return &ValidationError{"user_id", "is required"}
	case t.DriverID == uuid.Nil:
		return &ValidationError{"driver_id", "is required"}
	case !validCoordinates(t.StartLatitude, t.StartLongitude):
		return &ValidationError{"start_point", "is out of range"}
	case !validCoordinates(t.EndLatitude, t.EndLongitude):
		return &ValidationError{"end_point", "is out of range"}
	case t.Price < 0:
		return &ValidationError{"price", "must be >= 0"}
	}
	return nil
}

func validCoordinates(latitude, longitude float64) bool {
	return latitude >= -90 && latitude <= 90 &&
		longitude >= -180 && longitude <= 180
}
