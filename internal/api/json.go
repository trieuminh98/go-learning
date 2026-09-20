package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// DefaultMaxBodyBytes limits incoming JSON payloads to 1MB.
const DefaultMaxBodyBytes int64 = 1048576

// writeJSON serializes data to JSON, sets the Content-Type header to application/json,
// writes the HTTP status code, and outputs the response body.
func writeJSON(w http.ResponseWriter, status int, data any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		return fmt.Errorf("failed to encode json: %w", err)
	}
	return nil
}

// readJSON reads and strictly decodes a single JSON object from the request body into dst.
// It limits request size to 1MB, disallows unknown fields, formats decoding errors into
// client-friendly messages, and rejects trailing/multiple JSON values.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	// Guard against body larger than DefaultMaxBodyBytes
	r.Body = http.MaxBytesReader(w, r.Body, DefaultMaxBodyBytes)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(dst); err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("request body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("request body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("request body contains an invalid value for the %q field (at character %d)", unmarshalTypeError.Field, unmarshalTypeError.Offset)
			}
			return fmt.Errorf("request body contains an invalid value (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("request body must not be empty")

		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field ")
			return fmt.Errorf("request body contains unknown field %s", fieldName)

		case errors.As(err, &maxBytesError):
			return fmt.Errorf("request body must not be larger than %d bytes", maxBytesError.Limit)

		default:
			return err
		}
	}

	// Ensure the request body contains only a single JSON document.
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("request body must only contain a single JSON value")
	}

	return nil
}
