package respond

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

	api "github.com/maksimkasimovhse/TripGo/internal/generated"
)

func WriteInvalidRequest(w http.ResponseWriter, r *http.Request, detail string) {
	WriteProblem(w, r, http.StatusBadRequest, "invalid_request", "Invalid request", detail)
}

func WriteProblem(w http.ResponseWriter, r *http.Request, status int, code, title, detail string) {
	p := api.Problem{
		Type:   "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}

	WriteProblemBody(w, r, status, p)
}

func WriteProblemBody(w http.ResponseWriter, r *http.Request, status int, body api.Problem) {
	instance := r.URL.Path
	body.Instance = &instance

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("failed to write problem: %v", err)
	}
}
