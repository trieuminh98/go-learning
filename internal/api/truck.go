package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

// ListTrucksHandler lists all trucks.
func ListTrucksHandler(w http.ResponseWriter, r *http.Request) {
	trucks := logic.ListTrucks()
	writeJSON(w, http.StatusOK, trucks)
}

// GetTruckHandler translates HTTP requests to Logic calls and maps results to HTTP responses.
func GetTruckHandler(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.PathValue("id")
	}
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

// CreateTruckHandler parses payload and forwards to Logic layer.
func CreateTruckHandler(w http.ResponseWriter, r *http.Request) {
	var t core.Truck
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload: " + err.Error()})
		return
	}

	if err := logic.CreateTruck(t); err != nil {
		switch {
		case errors.Is(err, core.ErrDuplicateTruckID):
			writeJSON(w, http.StatusConflict, errorResponse{Error: err.Error()})
		case errors.Is(err, core.ErrInvalidTruckID),
			errors.Is(err, core.ErrInvalidCapacity),
			errors.Is(err, core.ErrOverloaded):
			writeJSON(w, http.StatusBadRequest, errorResponse{Error: err.Error()})
		default:
			writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "internal server error"})
		}
		return
	}

	writeJSON(w, http.StatusCreated, t)
}
