package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"

	"go-learning/docs"
	"go-learning/internal/api"
	"go-learning/internal/db"
	generated "go-learning/internal/db/generated"
	"go-learning/internal/env"
	"go-learning/internal/logic"
	"go-learning/internal/repository"
)

// DBConfig holds PostgreSQL connection pool configuration.
type DBConfig struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	MaxIdleTime time.Duration
	MaxLifetime time.Duration
}

// Config holds runtime configuration parameters for the application.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	DB           DBConfig
}

// Application holds runtime dependencies and application lifecycle methods.
type Application struct {
	config       Config
	userService  *logic.UserService
	truckService *logic.TruckService
	postService  *logic.PostService
	feedService  *logic.FeedService
	validator    *validator.Validate
}

// NewApplication constructs the Application root with injected dependencies.
func NewApplication(
	cfg Config,
	userService *logic.UserService,
	truckService *logic.TruckService,
	postService *logic.PostService,
	feedService *logic.FeedService,
	validator *validator.Validate,
) *Application {
	return &Application{
		config:       cfg,
		userService:  userService,
		truckService: truckService,
		postService:  postService,
		feedService:  feedService,
		validator:    validator,
	}
}

// routes builds and returns the application's HTTP handler tree.
// Notice: Returns http.Handler interface, completely decoupled from any concrete router type.
func (app *Application) routes() http.Handler {
	return api.NewRouter(api.RouterConfig{
		UserService:  app.userService,
		TruckService: app.truckService,
		PostService:  app.postService,
		FeedService:  app.feedService,
		Validator:    app.validator,
	})
}

// run configures and starts the HTTP server.
func (app *Application) run() error {
	srv := &http.Server{
		Addr:         app.config.Addr,
		Handler:      app.routes(),
		ReadTimeout:  app.config.ReadTimeout,
		WriteTimeout: app.config.WriteTimeout,
		IdleTimeout:  app.config.IdleTimeout,
	}

	log.Printf("Server listening on %s (ReadTimeout=%v, WriteTimeout=%v, IdleTimeout=%v)...",
		srv.Addr, srv.ReadTimeout, srv.WriteTimeout, srv.IdleTimeout)

	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// @title						Go Learning API
// @version					1.0
// @description				Go backend learning project demonstrating clean architecture, RESTful patterns, database indexing, and feed generation.
//
// @contact.name				API Support
// @contact.url				https://github.com/trieuminh98/go-learning
//
// @license.name				MIT
// @license.url				https://opensource.org/licenses/MIT
//
// @BasePath					/v1
//
// @securityDefinitions.apikey	BearerAuth
// @in							header
// @name						Authorization
// @description				Type "Bearer" followed by a space and JWT token.
func main() {
	// 1. Load runtime configuration
	cfg := Config{
		Addr:         env.GetString("SERVER_ADDR", ":8080"),
		ReadTimeout:  time.Duration(env.GetInt("SERVER_READ_TIMEOUT_SECONDS", 5)) * time.Second,
		WriteTimeout: time.Duration(env.GetInt("SERVER_WRITE_TIMEOUT_SECONDS", 10)) * time.Second,
		IdleTimeout:  time.Duration(env.GetInt("SERVER_IDLE_TIMEOUT_SECONDS", 120)) * time.Second,
		DB: DBConfig{
			URL:         env.GetString("DB_URL", "postgres://postgres:password@localhost:5433/golearning?sslmode=disable"),
			MaxConns:    int32(env.GetInt("DB_MAX_CONNS", 10)),
			MinConns:    int32(env.GetInt("DB_MIN_CONNS", 2)),
			MaxIdleTime: time.Duration(env.GetInt("DB_MAX_CONN_IDLE_TIME_SECONDS", 900)) * time.Second,
			MaxLifetime: time.Duration(env.GetInt("DB_MAX_CONN_LIFETIME_SECONDS", 3600)) * time.Second,
		},
	}

	// Configure Swagger host dynamically from runtime environment
	if apiHost := env.GetString("API_HOST", ""); apiHost != "" {
		docs.SwaggerInfo.Host = apiHost
	} else if cfg.Addr != "" && cfg.Addr[0] == ':' {
		docs.SwaggerInfo.Host = "localhost" + cfg.Addr
	}

	// 2. Initialize PostgreSQL connection pool with bounded startup timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pool, err := db.NewPool(
		ctx,
		cfg.DB.URL,
		cfg.DB.MaxConns,
		cfg.DB.MinConns,
		cfg.DB.MaxIdleTime,
		cfg.DB.MaxLifetime,
	)
	if err != nil {
		log.Fatalf("database connection failure: %v", err)
	}
	defer pool.Close() // Creator owns cleanup

	log.Printf("Connected to PostgreSQL pool (MaxConns=%d, MinConns=%d)", cfg.DB.MaxConns, cfg.DB.MinConns)

	// 3. Create sqlc Queries and wire PostgreSQL Storage aggregate
	queries := generated.New(pool)
	storage := repository.NewPostgresStorage(queries)

	// 4. Initialize logic services with only the specific repository each requires
	userService := logic.NewUserService(storage.Users, storage.Followers)
	truckService := logic.NewTruckService(storage.Trucks)
	postService := logic.NewPostService(storage.Posts, storage.Comments)
	feedService := logic.NewFeedService(storage.Feed)

	// 5. Initialize shared request validator
	validate := validator.New(validator.WithRequiredStructEnabled())

	// 6. Instantiate Application composition root
	app := NewApplication(cfg, userService, truckService, postService, feedService, validate)

	// 7. Start server
	if err := app.run(); err != nil {
		log.Fatalf("server failure: %v", err)
	}

}
