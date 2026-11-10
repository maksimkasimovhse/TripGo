package respond

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/maksimkasimovhse/TripGo/internal/generated"
)

func WriteInvalidRequest(w http.ResponseWriter, r *http.Request, logger *slog.Logger, detail string) {
	WriteProblem(w, r, logger, http.StatusBadRequest, "invalid_request", "Invalid request", detail)
}

func WriteProblem(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, code, title, detail string) {
	p := api.Problem{
		Type:   "https://tripgo.example/problems/" + strings.ReplaceAll(code, "_", "-"),
		Title:  title,
		Status: int32(status),
		Detail: &detail,
		Code:   code,
	}

	WriteProblemBody(w, r, logger, status, p)
}

func WriteProblemBody(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, body api.Problem) {
	instance := r.URL.Path
	body.Instance = &instance

	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		logger.ErrorContext(r.Context(), "write problem response failed", "error", err)
	}
}
