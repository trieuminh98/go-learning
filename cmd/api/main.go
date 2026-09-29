package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"
	goredis "github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"go-learning/docs"
	"go-learning/internal/api"
	"go-learning/internal/authenticator"
	"go-learning/internal/db"
	generated "go-learning/internal/db/generated"
	"go-learning/internal/env"
	"go-learning/internal/logic"
	"go-learning/internal/mailer"
	"go-learning/internal/repository"
	cacheRedis "go-learning/internal/repository/redis"
)

// DBConfig holds PostgreSQL connection pool configuration.
type DBConfig struct {
	URL         string
	MaxConns    int32
	MinConns    int32
	MaxIdleTime time.Duration
	MaxLifetime time.Duration
}

// AuthConfig holds authentication and token configuration.
type AuthConfig struct {
	TokenExpiry time.Duration
	BasicUser   string
	BasicPass   string
	JWTSecret   string
	JWTIssuer   string
	JWTAudience string
	JWTTTL      time.Duration
}

// RedisConfig holds Redis cache configuration.
type RedisConfig struct {
	Enabled      bool
	Addr         string
	Password     string
	DB           int
	UserCacheTTL time.Duration
}

// Config holds runtime configuration parameters for the application.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
	DB           DBConfig
	Auth         AuthConfig
	Redis        RedisConfig
}

// Application holds runtime dependencies and application lifecycle methods.
type Application struct {
	config        Config
	userService   *logic.UserService
	truckService  *logic.TruckService
	postService   *logic.PostService
	feedService   *logic.FeedService
	authenticator logic.Authenticator
	userRepo      logic.UserRepository
	roleRepo      logic.RoleRepository
	validator     *validator.Validate
	logger        *zap.SugaredLogger
}

// NewApplication constructs the Application root with injected dependencies.
func NewApplication(
	cfg Config,
	userService *logic.UserService,
	truckService *logic.TruckService,
	postService *logic.PostService,
	feedService *logic.FeedService,
	authenticator logic.Authenticator,
	userRepo logic.UserRepository,
	roleRepo logic.RoleRepository,
	validator *validator.Validate,
	logger *zap.SugaredLogger,
) *Application {
	return &Application{
		config:        cfg,
		userService:   userService,
		truckService:  truckService,
		postService:   postService,
		feedService:   feedService,
		authenticator: authenticator,
		userRepo:      userRepo,
		roleRepo:      roleRepo,
		validator:     validator,
		logger:        logger,
	}
}

// routes builds and returns the application's HTTP handler tree.
// Notice: Returns http.Handler interface, completely decoupled from any concrete router type.
func (app *Application) routes() http.Handler {
	return api.NewRouter(api.RouterConfig{
		UserService:   app.userService,
		TruckService:  app.truckService,
		PostService:   app.postService,
		FeedService:   app.feedService,
		Validator:     app.validator,
		Logger:        app.logger,
		BasicAuthUser: app.config.Auth.BasicUser,
		BasicAuthPass: app.config.Auth.BasicPass,
		Authenticator: app.authenticator,
		UserRepo:      app.userRepo,
		RoleRepo:      app.roleRepo,
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

	app.logger.Infow("server listening",
		"addr", srv.Addr,
		"read_timeout", srv.ReadTimeout,
		"write_timeout", srv.WriteTimeout,
		"idle_timeout", srv.IdleTimeout,
	)

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
//
// @securityDefinitions.basic	BasicAuth
func main() {
	// 1. Initialize Zap structured logger
	appEnv := env.GetString("APP_ENV", "development")
	var zapLogger *zap.Logger
	var err error
	if appEnv == "production" {
		zapLogger, err = zap.NewProduction()
	} else {
		zapLogger, err = zap.NewDevelopment()
	}
	if err != nil {
		log.Fatalf("failed to initialize zap logger: %v", err)
	}
	defer zapLogger.Sync()

	logger := zapLogger.Sugar().With("environment", appEnv)

	// 2. Load runtime configuration
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
		Auth: AuthConfig{
			TokenExpiry: time.Duration(env.GetInt("AUTH_TOKEN_EXPIRATION_HOURS", 72)) * time.Hour,
			BasicUser:   env.GetString("AUTH_BASIC_USER", "admin"),
			BasicPass:   env.GetString("AUTH_BASIC_PASS", "admin"),
			JWTSecret:   env.GetString("JWT_SECRET", "0123456789abcdef0123456789abcdef"),
			JWTIssuer:   env.GetString("JWT_ISSUER", "golearning"),
			JWTAudience: env.GetString("JWT_AUDIENCE", "golearning-users"),
			JWTTTL:      time.Duration(env.GetInt("JWT_TTL_MINUTES", 60)) * time.Minute,
		},
		Redis: RedisConfig{
			Enabled:      env.GetBool("REDIS_ENABLED", false),
			Addr:         env.GetString("REDIS_ADDR", "localhost:6379"),
			Password:     env.GetString("REDIS_PASSWORD", ""),
			DB:           env.GetInt("REDIS_DB", 0),
			UserCacheTTL: env.GetDuration("CACHE_USER_TTL", 15*time.Minute),
		},
	}

	if appEnv == "production" {
		if cfg.Auth.BasicUser == "admin" && cfg.Auth.BasicPass == "admin" {
			logger.Warnw("using default basic auth credentials in production environment is insecure")
		}
		if cfg.Auth.JWTSecret == "0123456789abcdef0123456789abcdef" {
			logger.Fatalw("cannot use default jwt secret in production environment")
		}
	}

	// Configure Swagger host dynamically from runtime environment
	if apiHost := env.GetString("API_HOST", ""); apiHost != "" {
		docs.SwaggerInfo.Host = apiHost
	} else if cfg.Addr != "" && cfg.Addr[0] == ':' {
		docs.SwaggerInfo.Host = "localhost" + cfg.Addr
	}

	// 3. Initialize PostgreSQL connection pool with bounded startup timeout
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
		logger.Fatalw("database connection failure", "error", err)
	}
	defer pool.Close() // Creator owns cleanup

	logger.Infow("connected to postgresql pool",
		"max_conns", cfg.DB.MaxConns,
		"min_conns", cfg.DB.MinConns,
	)

	// 4. Create sqlc Queries and wire PostgreSQL Storage aggregate
	queries := generated.New(pool)
	storage := repository.NewPostgresStorage(pool, queries)

	// 5. Initialize authenticators, mailer, optional redis cache, and logic services
	jwtAuth, err := authenticator.NewJWTAuthenticator(
		cfg.Auth.JWTSecret,
		cfg.Auth.JWTIssuer,
		cfg.Auth.JWTAudience,
		cfg.Auth.JWTTTL,
	)
	if err != nil {
		logger.Fatalw("failed to initialize jwt authenticator", "error", err)
	}

	var userCache logic.UserCache
	if cfg.Redis.Enabled {
		rdb := goredis.NewClient(&goredis.Options{
			Addr:         cfg.Redis.Addr,
			Password:     cfg.Redis.Password,
			DB:           cfg.Redis.DB,
			DialTimeout:  2 * time.Second,
			ReadTimeout:  150 * time.Millisecond,
			WriteTimeout: 150 * time.Millisecond,
		})

		pingCtx, pingCancel := context.WithTimeout(context.Background(), 2*time.Second)
		if err := rdb.Ping(pingCtx).Err(); err != nil {
			pingCancel()
			logger.Warnw("failed to connect to redis, caching will be disabled",
				"addr", cfg.Redis.Addr,
				"error", err,
			)
			_ = rdb.Close()
		} else {
			pingCancel()
			logger.Infow("connected to redis cache",
				"addr", cfg.Redis.Addr,
				"db", cfg.Redis.DB,
				"user_ttl", cfg.Redis.UserCacheTTL,
			)
			userCache = cacheRedis.NewUserCache(rdb, logger)
			defer rdb.Close()
		}
	} else {
		logger.Infow("redis cache is disabled (REDIS_ENABLED=false)")
	}

	devMailer := mailer.NewDevMailer()
	userService := logic.NewUserService(
		storage.Users,
		storage.Followers,
		storage.Invitations,
		storage.TxManager,
		devMailer,
		cfg.Auth.TokenExpiry,
		jwtAuth,
	)
	if userCache != nil {
		userService.WithCache(userCache, cfg.Redis.UserCacheTTL)
	}
	truckService := logic.NewTruckService(storage.Trucks)
	postService := logic.NewPostService(storage.Posts, storage.Comments)
	feedService := logic.NewFeedService(storage.Feed)

	// 6. Initialize shared request validator
	validate := validator.New(validator.WithRequiredStructEnabled())

	// 7. Instantiate Application composition root
	app := NewApplication(cfg, userService, truckService, postService, feedService, jwtAuth, storage.Users, storage.Roles, validate, logger)

	// 8. Start server
	if err := app.run(); err != nil {
		logger.Fatalw("server failure", "error", err)
	}
}
