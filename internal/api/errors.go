package api

import (
	"errors"
	"log"
	"net/http"

	"go-learning/internal/core"
)

// errorResponse defines the standardized JSON error envelope: {"error": "..."}.
type errorResponse struct {
	Error string `json:"error"`
}

// writeJSONError serializes an error message using the standard error envelope.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	if err := writeJSON(w, status, errorResponse{Error: message}); err != nil {
		log.Printf("failed to write json error response: %v", err)
	}
}

// MapDomainErrorToHTTP maps domain errors from internal/core to standardized HTTP status codes and responses.
func MapDomainErrorToHTTP(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, core.ErrInvalidFirstName),
		errors.Is(err, core.ErrInvalidLastName),
		errors.Is(err, core.ErrInvalidTruckID),
		errors.Is(err, core.ErrInvalidCapacity),
		errors.Is(err, core.ErrOverloaded):
		writeJSONError(w, http.StatusBadRequest, err.Error())

	case errors.Is(err, core.ErrUserAlreadyExists),
		errors.Is(err, core.ErrDuplicateTruckID):
		writeJSONError(w, http.StatusConflict, err.Error())

	case errors.Is(err, core.ErrTruckNotFound):
		writeJSONError(w, http.StatusNotFound, err.Error())

	default:
		writeJSONError(w, http.StatusInternalServerError, "internal server error")
	}
}
