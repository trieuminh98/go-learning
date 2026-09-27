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
	Users     logic.UserRepository
	Trucks    logic.TruckRepository
	Posts     logic.PostRepository
	Comments  logic.CommentRepository
	Followers logic.FollowerRepository
}

// NewStorage initializes and bundles concrete in-memory repository implementations.
func NewStorage() Storage {
	return Storage{
		Users:     memory.NewUserRepository(),
		Trucks:    memory.NewTruckRepository(),
		Posts:     memory.NewPostRepository(),
		Comments:  memory.NewCommentRepository(),
		Followers: memory.NewFollowerRepository(),
	}
}

// NewPostgresStorage initializes and bundles PostgreSQL-backed repository implementations using sqlc queries.
func NewPostgresStorage(queries *db.Queries) Storage {
	return Storage{
		Users:     postgres.NewUserRepository(queries),
		Trucks:    postgres.NewTruckRepository(queries),
		Posts:     postgres.NewPostRepository(queries),
		Comments:  postgres.NewCommentRepository(queries),
		Followers: postgres.NewFollowerRepository(queries),
	}
}
