package repository

import (
	"go-learning/internal/logic"
	"go-learning/internal/repository/memory"
)

// Storage is a composition aggregate holding all domain repository interfaces.
// It simplifies dependency bootstrapping at the composition root without coupling
// consumer layers to repositories they do not need.
type Storage struct {
	Users  logic.UserRepository
	Trucks logic.TruckRepository
}

// NewStorage initializes and bundles concrete repository implementations.
// For now, it wires thread-safe in-memory repositories.
func NewStorage() Storage {
	return Storage{
		Users:  memory.NewUserRepository(),
		Trucks: memory.NewTruckRepository(),
	}
}
