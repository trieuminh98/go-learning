package api

import (
	"errors"
	"fmt"
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

// internalServerError logs the detailed error with HTTP request context and returns a safe 500 error response.
func internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	reqID := r.Header.Get("X-Request-ID")
	if reqID != "" {
		log.Printf("[ERROR] method=%s path=%s request_id=%s error=%v", r.Method, r.URL.Path, reqID, err)
	} else {
		log.Printf("[ERROR] method=%s path=%s error=%v", r.Method, r.URL.Path, err)
	}

	writeJSONError(w, http.StatusInternalServerError, "the server encountered a problem")
}

// badRequest sends a 400 Bad Request response with a safe, actionable client error message.
func badRequest(w http.ResponseWriter, r *http.Request, err error) {
	writeJSONError(w, http.StatusBadRequest, err.Error())
}

// notFound sends a 404 Not Found response.
func notFound(w http.ResponseWriter, r *http.Request, message ...string) {
	msg := "resource not found"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}
	writeJSONError(w, http.StatusNotFound, msg)
}

// conflict sends a 409 Conflict response.
func conflict(w http.ResponseWriter, r *http.Request, message string) {
	writeJSONError(w, http.StatusConflict, message)
}

// unauthorized sends a 401 Unauthorized response.
func unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	writeJSONError(w, http.StatusUnauthorized, message)
}

// MapDomainErrorToHTTP maps domain errors from internal/core to standardized HTTP status codes and responses.
func MapDomainErrorToHTTP(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, core.ErrInvalidFirstName),
		errors.Is(err, core.ErrInvalidLastName),
		errors.Is(err, core.ErrInvalidTruckID),
		errors.Is(err, core.ErrInvalidCapacity),
		errors.Is(err, core.ErrOverloaded),
		errors.Is(err, core.ErrInvalidTitle),
		errors.Is(err, core.ErrInvalidContent),
		errors.Is(err, core.ErrCannotFollowSelf):
		badRequest(w, r, err)

	case errors.Is(err, core.ErrUserAlreadyExists),
		errors.Is(err, core.ErrDuplicateTruckID),
		errors.Is(err, core.ErrConflict):
		conflict(w, r, err.Error())

	case errors.Is(err, core.ErrTruckNotFound),
		errors.Is(err, core.ErrUserNotFound),
		errors.Is(err, core.ErrPostNotFound):
		notFound(w, r, err.Error())

	default:
		internalServerError(w, r, fmt.Errorf("unmapped domain error: %w", err))
	}
}
