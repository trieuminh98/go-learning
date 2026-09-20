package main

import (
	"errors"
	"log"
	"net/http"
	"time"

	"go-learning/internal/api"
	"go-learning/internal/env"
)

// Config holds runtime configuration parameters for the application.
type Config struct {
	Addr         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
	IdleTimeout  time.Duration
}

// Application holds runtime dependencies and application lifecycle methods.
type Application struct {
	config Config
}

// routes builds and returns the application's HTTP handler tree.
// Notice: Returns http.Handler interface, completely decoupled from any concrete router type.
func (app *Application) routes() http.Handler {
	return api.NewRouter()
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

func main() {
	cfg := Config{
		Addr:         env.GetString("SERVER_ADDR", ":8080"),
		ReadTimeout:  time.Duration(env.GetInt("SERVER_READ_TIMEOUT_SECONDS", 5)) * time.Second,
		WriteTimeout: time.Duration(env.GetInt("SERVER_WRITE_TIMEOUT_SECONDS", 10)) * time.Second,
		IdleTimeout:  time.Duration(env.GetInt("SERVER_IDLE_TIMEOUT_SECONDS", 120)) * time.Second,
	}

	app := &Application{
		config: cfg,
	}

	if err := app.run(); err != nil {
		log.Fatalf("server failure: %v", err)
	}
}
