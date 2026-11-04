package trip

import (
	"encoding/json"
	"github.com/google/uuid"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func validateTripData(d api.TripData) string {
	switch {
	case d.UserId == uuid.Nil:
		return "user_id is required"
	case d.DriverId == uuid.Nil:
		return "driver_id is required"
	case !validCoordinates(d.StartPoint):
		return "start_point is out of range"
	case !validCoordinates(d.EndPoint):
		return "end_point is out of range"
	case d.Price < 0:
		return "price must be >= 0"
	}
	return ""
}

const MaxBodyBytes = 1 << 20

func checkRequiredFields(body []byte) string {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(body, &top); err != nil {
		return "Request body must be a JSON object"
	}
	for _, f := range []string{"user_id", "driver_id", "start_point", "end_point", "price"} {
		if v, ok := top[f]; !ok || string(v) == "null" {
			return f + " is required"
		}
	}
	for _, p := range []string{"start_point", "end_point"} {
		var point map[string]json.RawMessage
		if err := json.Unmarshal(top[p], &point); err != nil {
			return p + " must be an object"
		}
		for _, f := range []string{"latitude", "longitude"} {
			if v, ok := point[f]; !ok || string(v) == "null" {
				return p + "." + f + " is required"
			}
		}
	}
	return ""
}

func validCoordinates(c api.Coordinates) bool {
	return c.Latitude >= -90 && c.Latitude <= 90 &&
		c.Longitude >= -180 && c.Longitude <= 180
}
