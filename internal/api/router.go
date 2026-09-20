package api

import "net/http"

// NewRouter registers all API layer endpoints into a ServeMux.
func NewRouter() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /trucks/{id}", GetTruckHandler)
	return mux
}
