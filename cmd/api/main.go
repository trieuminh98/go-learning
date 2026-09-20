package main

import (
	"errors"
	"log"
	"net/http"

	"go-learning/internal/api"
	"go-learning/internal/gateway"
)

func main() {
	// 1. Setup API router
	apiRouter := api.NewRouter()

	// 2. Wrap with Gateway middleware stack (RequestID -> Logging -> Auth -> API)
	gatewayHandler := gateway.RequestIDMiddleware(
		gateway.LoggingMiddleware(
			gateway.AuthMiddleware(apiRouter),
		),
	)

	// 3. Start server
	addr := ":8080"
	log.Printf("Server listening on %s...", addr)
	if err := http.ListenAndServe(addr, gatewayHandler); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("server failure: %v", err)
	}
}
