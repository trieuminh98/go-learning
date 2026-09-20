package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"go-learning/internal/gateway"
)

// NewRouter builds and returns the application HTTP handler using Chi.
// Return type is http.Handler, keeping http.Server decoupled from Chi.
func NewRouter() http.Handler {
	r := chi.NewRouter()

	// 1. Gateway global cross-cutting middleware stack
	r.Use(gateway.RecoveryMiddleware)
	r.Use(gateway.RequestIDMiddleware)
	r.Use(gateway.LoggingMiddleware)
	r.Use(gateway.TimeoutMiddleware(10 * time.Second))

	// Root welcome route
	r.Get("/", WelcomeHandler)

	// 2. Versioned API routes under /v1 prefix
	r.Route("/v1", func(v1 chi.Router) {
		// Public route group: no authentication required
		v1.Group(func(public chi.Router) {
			public.Get("/health", HealthHandler)
			public.Get("/debug/panic", func(w http.ResponseWriter, r *http.Request) {
				panic("deliberate test panic for recovery middleware verification")
			})
		})

		// Private route group: protected by Gateway AuthMiddleware
		v1.Group(func(private chi.Router) {
			private.Use(gateway.AuthMiddleware)

			// Example protected route
			private.Get("/private/profile", ProtectedExampleHandler)

			// Trucks resources
			private.Get("/trucks", ListTrucksHandler)
			private.Get("/trucks/{id}", GetTruckHandler)
			private.Post("/trucks", CreateTruckHandler)

			// Users resources
			private.Get("/users", ListUsersHandler)
			private.Post("/users", CreateUserHandler)
		})
	})

	return r
}
