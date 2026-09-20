package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

type errorResponse struct {
	Error string `json:"error"`
}

// GetTruckHandler translates HTTP requests to Logic calls and maps results to HTTP responses.
func GetTruckHandler(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing truck id in path"})
		return
	}

	truck, err := logic.GetTruck(id)
	if err != nil {
		switch {
		case errors.Is(err, core.ErrInvalidTruckID):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		case errors.Is(err, core.ErrTruckNotFound):
			writeJSON(w, http.StatusNotFound, errorResponse{Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusOK, truck)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
