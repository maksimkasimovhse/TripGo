package create

import (
	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	createuc "github.com/maksimkasimovhse/TripGo/internal/usecase/trip/create"
)

func ToCreateInput(req api.TripData, tripID uuid.UUID) createuc.Input {
	return createuc.Input{
		ID:             tripID,
		UserID:         req.UserId,
		DriverID:       req.DriverId,
		StartLatitude:  req.StartPoint.Latitude,
		StartLongitude: req.StartPoint.Longitude,
		EndLatitude:    req.EndPoint.Latitude,
		EndLongitude:   req.EndPoint.Longitude,
		Price:          req.Price,
	}
}
