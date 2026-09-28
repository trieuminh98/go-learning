package api

import (
	"net/http"
)

// HealthResponse represents the response payload for the health check endpoint.
type HealthResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
}

type healthResponse = HealthResponse

// HealthHandler returns API service availability. Public endpoint without authentication.
//
//	@Summary		Health check
//	@Description	Returns service status, environment, and version
//	@Tags			system
//	@Produce		json
//	@Success		200	{object}	HealthResponse
//	@Router			/health [get]
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	_ = jsonResponse(w, http.StatusOK, HealthResponse{
		Status:      "available",
		Environment: "development",
		Version:     "1.0.0",
	})
}

// ProtectedExampleHandler serves as a verification endpoint for private route groups.
func ProtectedExampleHandler(w http.ResponseWriter, r *http.Request) {
	_ = jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Access granted to protected resource",
		"status":  "authenticated",
	})
}
