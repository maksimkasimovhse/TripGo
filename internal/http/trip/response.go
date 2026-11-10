package trip

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
)

func ParamErrorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var (
			invalidFormat  *api.InvalidParamFormatError
			requiredParam  *api.RequiredParamError
			requiredHeader *api.RequiredHeaderError
		)

		switch {
		case errors.As(err, &invalidFormat):
			respond.WriteInvalidRequest(w, r, logger, "parameter "+invalidFormat.ParamName+" has invalid format")
		case errors.As(err, &requiredParam):
			respond.WriteInvalidRequest(w, r, logger, "parameter "+requiredParam.ParamName+" is required")
		case errors.As(err, &requiredHeader):
			respond.WriteInvalidRequest(w, r, logger, "header "+requiredHeader.ParamName+" is required")
		default:
			respond.WriteInvalidRequest(w, r, logger, "invalid request parameters")
		}
	}
}

const tripsPath = "/api/v1/trips/"

func TripLocation(id uuid.UUID) string {
	return tripsPath + id.String()
}
