package api

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

// TruckHandler handles HTTP truck requests and maps them to logic.TruckService.
type TruckHandler struct {
	service *logic.TruckService
}

// NewTruckHandler creates a new TruckHandler.
func NewTruckHandler(service *logic.TruckService) *TruckHandler {
	return &TruckHandler{service: service}
}

// ListTrucks lists all trucks.
func (h *TruckHandler) ListTrucks(w http.ResponseWriter, r *http.Request) {
	trucks, err := h.service.ListTrucks(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, errorResponse{Error: "failed to list trucks"})
		return
	}
	writeJSON(w, http.StatusOK, trucks)
}

// GetTruck translates HTTP requests to TruckService calls.
func (h *TruckHandler) GetTruck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "missing truck id in path"})
		return
	}

	truck, err := h.service.GetTruck(r.Context(), id)
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

// CreateTruck parses payload and forwards to TruckService.
func (h *TruckHandler) CreateTruck(w http.ResponseWriter, r *http.Request) {
	var t core.Truck
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse{Error: "invalid json payload: " + err.Error()})
		return
	}

	if err := h.service.CreateTruck(r.Context(), t); err != nil {
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
