package api

import (
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

// CreateTruckRequest is the API DTO for incoming truck creation.
type CreateTruckRequest struct {
	ID          string `json:"id"`
	Capacity    int    `json:"capacity"`
	CurrentLoad int    `json:"current_load"`
}

// TruckResponse is the API DTO for outgoing truck responses.
type TruckResponse struct {
	ID          string `json:"id"`
	Capacity    int    `json:"capacity"`
	CurrentLoad int    `json:"current_load"`
}

func toTruckResponse(t core.Truck) TruckResponse {
	return TruckResponse{
		ID:          t.ID,
		Capacity:    t.Capacity,
		CurrentLoad: t.CurrentLoad,
	}
}

// ListTrucks lists all trucks.
func (h *TruckHandler) ListTrucks(w http.ResponseWriter, r *http.Request) {
	trucks, err := h.service.ListTrucks(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to list trucks")
		return
	}

	resp := make([]TruckResponse, 0, len(trucks))
	for _, t := range trucks {
		resp = append(resp, toTruckResponse(t))
	}
	_ = writeJSON(w, http.StatusOK, resp)
}

// GetTruck translates HTTP requests to TruckService calls.
func (h *TruckHandler) GetTruck(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.PathValue("id")
	}
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing truck id in path")
		return
	}

	truck, err := h.service.GetTruck(r.Context(), id)
	if err != nil {
		MapDomainErrorToHTTP(w, err)
		return
	}

	_ = writeJSON(w, http.StatusOK, toTruckResponse(truck))
}

// CreateTruck parses payload and forwards to TruckService.
func (h *TruckHandler) CreateTruck(w http.ResponseWriter, r *http.Request) {
	var input CreateTruckRequest
	if err := readJSON(w, r, &input); err != nil {
		writeJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	entity := core.Truck{
		ID:          input.ID,
		Capacity:    input.Capacity,
		CurrentLoad: input.CurrentLoad,
	}

	if err := h.service.CreateTruck(r.Context(), entity); err != nil {
		MapDomainErrorToHTTP(w, err)
		return
	}

	_ = writeJSON(w, http.StatusCreated, toTruckResponse(entity))
}
