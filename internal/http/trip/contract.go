package trip

import (
	"net/http"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

type createTrip interface {
	CreateTrip(w http.ResponseWriter, r *http.Request, params api.CreateTripParams)
}

type finishTrip interface {
	FinishTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId)
}

type getTrip interface {
	GetTrip(w http.ResponseWriter, r *http.Request, tripID api.TripId)
}
