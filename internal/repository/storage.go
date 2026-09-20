package repository

import (
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
	"go-learning/internal/repository/memory"
	"go-learning/internal/repository/postgres"
)

// Storage is a composition aggregate holding all domain repository interfaces.
// It simplifies dependency bootstrapping at the composition root without coupling
// consumer layers to repositories they do not need.
type Storage struct {
	Users  logic.UserRepository
	Trucks logic.TruckRepository
}

// NewStorage initializes and bundles concrete in-memory repository implementations.
func NewStorage() Storage {
	return Storage{
		Users:  memory.NewUserRepository(),
		Trucks: memory.NewTruckRepository(),
	}
}

// NewPostgresStorage initializes and bundles PostgreSQL-backed repository implementations using sqlc queries.
func NewPostgresStorage(queries *db.Queries) Storage {
	return Storage{
		Users:  postgres.NewUserRepository(queries),
		Trucks: postgres.NewTruckRepository(queries),
	}
}
