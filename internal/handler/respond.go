package handler

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/domain"
	api "github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/repository"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func writeProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	instance := r.URL.Path
	p := api.Problem{
		Type:     "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:    title,
		Status:   int32(status),
		Detail:   &detail,
		Instance: &instance,
		Code:     code,
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(p); err != nil {
		log.Printf("failed to write problem: %v", err)
	}
}

func writeInvalidRequest(w http.ResponseWriter, r *http.Request, detail string) {
	writeProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", detail)
}

func ParamErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	writeInvalidRequest(w, r, err.Error())
}

func writeCreateTripResponse(w http.ResponseWriter, r *http.Request, tripRepo *repository.TripRepository, status int, tripID uuid.UUID) {
	trip, err := tripRepo.GetByID(r.Context(), tripID)
	if err != nil {
		if errors.Is(err, domain.ErrTripNotFound) {
			log.Printf("idempotent response: trip %s marked completed but not found: %v", tripID, err)
			writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
			return
		}
		log.Printf("load trip for idempotent response: %v", err)
		writeProblem(w, r, http.StatusInternalServerError, "internal_error", "Internal Server Error", "Internal server error")
		return
	}

	w.Header().Set("Location", tripLocation(trip.ID))
	writeJSON(w, status, toAPITrip(trip))
}

const tripsPath = "/api/v1/trips/"

func tripLocation(id uuid.UUID) string {
	return tripsPath + id.String()
}
