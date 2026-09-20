package api

import "net/http"

// NewRouter registers all API layer endpoints into a ServeMux.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", WelcomeHandler)
	mux.HandleFunc("GET /trucks", ListTrucksHandler)
	mux.HandleFunc("GET /trucks/{id}", GetTruckHandler)
	mux.HandleFunc("POST /trucks", CreateTruckHandler)
	return mux
}
