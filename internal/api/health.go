package api

import (
	"net/http"
)

type healthResponse struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Version     string `json:"version"`
}

// HealthHandler returns API service availability. Public endpoint without authentication.
func HealthHandler(w http.ResponseWriter, r *http.Request) {
	_ = jsonResponse(w, http.StatusOK, healthResponse{
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
