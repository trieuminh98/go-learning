package api

import (
	"errors"
	"fmt"
	"net/http"

	"go.uber.org/zap"

	"go-learning/internal/core"
	"go-learning/internal/gateway"
)

// ErrorResponse defines the standardized JSON error envelope: {"error": "..."}.
type ErrorResponse struct {
	Error string `json:"error"`
}

type errorResponse = ErrorResponse

var apiLogger *zap.SugaredLogger

// SetLogger configures the structured logger used by API error handlers.
func SetLogger(l *zap.SugaredLogger) {
	apiLogger = l
}

func extractLogFields(r *http.Request, err error) []any {
	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = gateway.RequestIDFromContext(r.Context())
	}

	fields := []any{
		"method", r.Method,
		"path", r.URL.Path,
		"request_id", reqID,
	}

	if uid, err := UserIDFromContext(r.Context()); err == nil && uid > 0 {
		fields = append(fields, "user_id", uid)
	}

	if err != nil {
		fields = append(fields, "error", err.Error())
	}

	return fields
}

// writeJSONError serializes an error message using the standard error envelope.
func writeJSONError(w http.ResponseWriter, status int, message string) {
	if err := writeJSON(w, status, ErrorResponse{Error: message}); err != nil {
		if apiLogger != nil {
			apiLogger.Errorw("failed to write json error response", "error", err.Error())
		}
	}
}

// internalServerError logs the detailed error with HTTP request context and returns a safe 500 error response.
func internalServerError(w http.ResponseWriter, r *http.Request, err error) {
	if apiLogger != nil {
		fields := extractLogFields(r, err)
		apiLogger.Errorw("internal server error", fields...)
	}

	writeJSONError(w, http.StatusInternalServerError, "the server encountered a problem")
}

// badRequest sends a 400 Bad Request response with a safe, actionable client error message.
func badRequest(w http.ResponseWriter, r *http.Request, err error) {
	if apiLogger != nil {
		fields := extractLogFields(r, err)
		apiLogger.Warnw("bad request", fields...)
	}

	writeJSONError(w, http.StatusBadRequest, err.Error())
}

// notFound sends a 404 Not Found response.
func notFound(w http.ResponseWriter, r *http.Request, message ...string) {
	msg := "resource not found"
	if len(message) > 0 && message[0] != "" {
		msg = message[0]
	}

	if apiLogger != nil {
		fields := extractLogFields(r, errors.New(msg))
		apiLogger.Infow("resource not found", fields...)
	}

	writeJSONError(w, http.StatusNotFound, msg)
}

// conflict sends a 409 Conflict response.
func conflict(w http.ResponseWriter, r *http.Request, message string) {
	if apiLogger != nil {
		fields := extractLogFields(r, errors.New(message))
		apiLogger.Warnw("conflict", fields...)
	}

	writeJSONError(w, http.StatusConflict, message)
}

// unauthorized sends a 401 Unauthorized response.
func unauthorized(w http.ResponseWriter, r *http.Request, message string) {
	if apiLogger != nil {
		fields := extractLogFields(r, errors.New(message))
		apiLogger.Warnw("unauthorized", fields...)
	}

	writeJSONError(w, http.StatusUnauthorized, message)
}

// MapDomainErrorToHTTP maps domain errors from internal/core to standardized HTTP status codes and responses.
func MapDomainErrorToHTTP(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, core.ErrInvalidFirstName),
		errors.Is(err, core.ErrInvalidLastName),
		errors.Is(err, core.ErrInvalidEmail),
		errors.Is(err, core.ErrInvalidPassword),
		errors.Is(err, core.ErrInvalidTruckID),
		errors.Is(err, core.ErrInvalidCapacity),
		errors.Is(err, core.ErrOverloaded),
		errors.Is(err, core.ErrInvalidTitle),
		errors.Is(err, core.ErrInvalidContent),
		errors.Is(err, core.ErrCannotFollowSelf):
		badRequest(w, r, err)

	case errors.Is(err, core.ErrUserAlreadyExists),
		errors.Is(err, core.ErrEmailAlreadyExists),
		errors.Is(err, core.ErrDuplicateTruckID),
		errors.Is(err, core.ErrConflict):
		conflict(w, r, err.Error())

	case errors.Is(err, core.ErrTruckNotFound),
		errors.Is(err, core.ErrUserNotFound),
		errors.Is(err, core.ErrInvitationNotFound),
		errors.Is(err, core.ErrPostNotFound):
		notFound(w, r, err.Error())

	case errors.Is(err, core.ErrInvalidCredentials):
		unauthorized(w, r, "invalid credentials")

	default:
		internalServerError(w, r, fmt.Errorf("unmapped domain error: %w", err))
	}
}
