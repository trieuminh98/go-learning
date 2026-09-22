package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"go-learning/internal/gateway"
	"go-learning/internal/logic"
)

// RouterConfig bundles the handler dependencies needed to build the API route tree.
type RouterConfig struct {
	UserService  *logic.UserService
	TruckService *logic.TruckService
	PostService  *logic.PostService
}

// NewRouter builds and returns the application HTTP handler using Chi.
// Return type is http.Handler, keeping http.Server decoupled from Chi.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	userHandler := NewUserHandler(cfg.UserService)
	truckHandler := NewTruckHandler(cfg.TruckService)
	postHandler := NewPostHandler(cfg.PostService)

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
			private.Get("/trucks", truckHandler.ListTrucks)
			private.Get("/trucks/{id}", truckHandler.GetTruck)
			private.Post("/trucks", truckHandler.CreateTruck)

			// Users resources
			private.Get("/users", userHandler.ListUsers)
			private.Post("/users", userHandler.CreateUser)

			// Posts resources
			private.Get("/posts", postHandler.ListPosts)
			private.Get("/posts/{postID}", postHandler.GetPost)
			private.Post("/posts", postHandler.CreatePost)
		})
	})



	return r
}
