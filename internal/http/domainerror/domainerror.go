package domainerror

import (
	"errors"
	"net/http"
	"strings"

	"github.com/maksimkasimovhse/TripGo/internal/domain/idempotency"
	"github.com/maksimkasimovhse/TripGo/internal/domain/trip"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func Problem(err error) (int, api.Problem) {
	var validationErr *trip.ValidationError

	switch {
	case errors.As(err, &validationErr):
		return makeProblem(
			http.StatusBadRequest,
			"invalid_request",
			"Invalid request",
			validationErr.Error(),
		)

	case errors.Is(err, trip.ErrTripNotFound):
		return makeProblem(
			http.StatusNotFound,
			"trip_not_found",
			"Trip not found",
			"Trip was not found",
		)

	case errors.Is(err, trip.ErrDriverBusy):
		return makeProblem(
			http.StatusConflict,
			"driver_busy",
			"Driver busy",
			"Driver already has an active trip",
		)

	case errors.Is(err, trip.ErrTripCompleted):
		return makeProblem(
			http.StatusConflict,
			"trip_completed",
			"Trip completed",
			"Operation is not allowed for a completed trip",
		)

	case errors.Is(err, idempotency.ErrIdempotencyKeyConflict):
		return makeProblem(
			http.StatusConflict,
			"idempotency_conflict",
			"Idempotency conflict",
			"Idempotency-Key was already used with a different request body",
		)

	case errors.Is(err, idempotency.ErrRequestInProgress):
		return makeProblem(
			http.StatusConflict,
			"request_in_progress",
			"Request In Progress",
			"Original request is still processing",
		)

	default:
		return makeProblem(
			http.StatusInternalServerError,
			"internal_error",
			"Internal Server Error",
			"Internal server error",
		)
	}
}

func makeProblem(status int, code, title, detail string) (int, api.Problem) {
	return status, api.Problem{
		Type:   "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}
}
