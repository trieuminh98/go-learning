package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/go-playground/validator/v10"
	httpSwagger "github.com/swaggo/http-swagger/v2"
	"go.uber.org/zap"

	"go-learning/internal/gateway"
	"go-learning/internal/logic"
)

// RouterConfig bundles the handler dependencies needed to build the API route tree.
type RouterConfig struct {
	UserService   *logic.UserService
	TruckService  *logic.TruckService
	PostService   *logic.PostService
	FeedService   *logic.FeedService
	Validator     *validator.Validate
	Logger        *zap.SugaredLogger
	BasicAuthUser string
	BasicAuthPass string
	Authenticator logic.Authenticator
	UserRepo      logic.UserRepository
	RoleRepo      logic.RoleRepository
	RateLimiter   gateway.RateLimiter
	CORSOrigins   []string
}

// NewRouter builds and returns the application HTTP handler using Chi.
// Return type is http.Handler, keeping http.Server decoupled from Chi.
func NewRouter(cfg RouterConfig) http.Handler {
	r := chi.NewRouter()

	if cfg.Logger != nil {
		SetLogger(cfg.Logger)
	}

	basicAuthMiddleware := gateway.BasicAuthMiddleware(cfg.BasicAuthUser, cfg.BasicAuthPass)
	var userProvider logic.UserGetter = cfg.UserService
	if userProvider == nil {
		userProvider = cfg.UserRepo
	}
	jwtAuthMiddleware := gateway.JWTAuthMiddleware(cfg.Authenticator, userProvider)

	userHandler := NewUserHandler(cfg.UserService, cfg.Validator)
	truckHandler := NewTruckHandler(cfg.TruckService)
	postHandler := NewPostHandler(cfg.PostService, cfg.Validator, cfg.RoleRepo)
	feedHandler := NewFeedHandler(cfg.FeedService)

	// 1. Gateway global cross-cutting middleware stack
	r.Use(gateway.RecoveryMiddleware(cfg.Logger))
	r.Use(gateway.RequestIDMiddleware)

	// 2. CORS middleware (runs before RateLimit & Auth so preflights do not exhaust quota and 429 carries CORS headers)
	if len(cfg.CORSOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   cfg.CORSOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Request-ID"},
			ExposedHeaders:   []string{"Link", "Retry-After", "X-Request-ID"},
			AllowCredentials: false,
			MaxAge:           300,
		}))
	}

	r.Use(gateway.RateLimitMiddleware(cfg.RateLimiter, cfg.Logger))
	r.Use(gateway.LoggingMiddleware(cfg.Logger))
	r.Use(gateway.TimeoutMiddleware(10 * time.Second))

	// Root welcome route
	r.Get("/", WelcomeHandler)

	// 2. Versioned API routes under /v1 prefix
	r.Route("/v1", func(v1 chi.Router) {
		// Tooling route group: protected by Gateway BasicAuthMiddleware
		v1.Group(func(tooling chi.Router) {
			tooling.Use(basicAuthMiddleware)

			tooling.Get("/health", HealthHandler)
			tooling.Get("/swagger/*", httpSwagger.WrapHandler)
			tooling.Get("/debug/panic", func(w http.ResponseWriter, r *http.Request) {
				panic("deliberate test panic for recovery middleware verification")
			})
			tooling.Get("/debug/slow", func(w http.ResponseWriter, r *http.Request) {
				// Deliberately delay response to simulate in-flight request during graceful shutdown test
				time.Sleep(3 * time.Second)
				_ = jsonResponse(w, http.StatusOK, map[string]string{
					"status":  "completed",
					"message": "request finished after artificial delay",
				})
			})
		})

		// Public route group: no authentication required
		v1.Group(func(public chi.Router) {
			// Authentication / Registration / Activation / Token
			public.Route("/authentication", func(auth chi.Router) {
				auth.Post("/user", userHandler.RegisterUser)
				auth.Put("/users/activate/{token}", userHandler.ActivateUser)
				auth.Put("/user/activate/{token}", userHandler.ActivateUser)
				auth.Post("/token", userHandler.CreateToken)
			})
		})

		// Private route group: protected by Gateway JWTAuthMiddleware
		v1.Group(func(private chi.Router) {
			private.Use(jwtAuthMiddleware)

			// Example protected route
			private.Get("/private/profile", ProtectedExampleHandler)

			// Trucks resources
			private.Get("/trucks", truckHandler.ListTrucks)
			private.Get("/trucks/{id}", truckHandler.GetTruck)
			private.Post("/trucks", truckHandler.CreateTruck)

			// Users resources
			private.Get("/users", userHandler.ListUsers)
			private.Get("/users/{userID}", userHandler.GetUser)
			private.Post("/users", userHandler.CreateUser)
			private.Put("/users/{userID}/follow", userHandler.Follow)
			private.Put("/users/{userID}/unfollow", userHandler.Unfollow)

			// Posts resources
			private.Get("/posts", postHandler.ListPosts)
			private.Post("/posts", postHandler.CreatePost)
			private.Get("/posts/{postID}", postHandler.GetPost)
			private.With(postHandler.PostContextMiddleware, postHandler.AuthorizePost("moderator")).Patch("/posts/{postID}", postHandler.UpdatePost)
			private.With(postHandler.PostContextMiddleware, postHandler.AuthorizePost("admin")).Delete("/posts/{postID}", postHandler.DeletePost)

			// Feed resources
			private.Get("/feed", feedHandler.GetFeed)
		})
	})

	return r
}
