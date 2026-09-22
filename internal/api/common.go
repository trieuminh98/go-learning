package api

import (
	"net/http"
)

// WelcomeHandler handles root health / welcome requests.
func WelcomeHandler(w http.ResponseWriter, r *http.Request) {
	_ = jsonResponse(w, http.StatusOK, map[string]string{
		"message": "Welcome to Truck Management API",
		"status":  "healthy",
	})
}
