package trip

import (
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"
	"github.com/maksimkasimovhse/TripGo/internal/generated"
	"github.com/maksimkasimovhse/TripGo/internal/http/respond"
)

func ParamErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	log.Printf("invalid request parameter: %s %s: %v", r.Method, r.URL.Path, err)

	var (
		invalidFormat  *api.InvalidParamFormatError
		requiredParam  *api.RequiredParamError
		requiredHeader *api.RequiredHeaderError
	)

	switch {
	case errors.As(err, &invalidFormat):
		respond.WriteInvalidRequest(w, r, "parameter "+invalidFormat.ParamName+" has invalid format")
	case errors.As(err, &requiredParam):
		respond.WriteInvalidRequest(w, r, "parameter "+requiredParam.ParamName+" is required")
	case errors.As(err, &requiredHeader):
		respond.WriteInvalidRequest(w, r, "header "+requiredHeader.ParamName+" is required")
	default:
		respond.WriteInvalidRequest(w, r, "invalid request parameters")
	}
}

const tripsPath = "/api/v1/trips/"

func TripLocation(id uuid.UUID) string {
	return tripsPath + id.String()
}
