package repository

import (
	"github.com/jackc/pgx/v5/pgxpool"

	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
	"go-learning/internal/repository/memory"
	"go-learning/internal/repository/postgres"
)

// Storage is a composition aggregate holding all domain repository interfaces and transaction runners.
// It simplifies dependency bootstrapping at the composition root without coupling
// consumer layers to repositories they do not need.
type Storage struct {
	Users       logic.UserRepository
	Trucks      logic.TruckRepository
	Posts       logic.PostRepository
	Comments    logic.CommentRepository
	Followers   logic.FollowerRepository
	Feed        logic.FeedRepository
	Invitations logic.InvitationRepository
	TxManager   logic.TransactionManager
}

// NewStorage initializes and bundles concrete in-memory repository implementations.
func NewStorage() Storage {
	users := memory.NewUserRepository()
	trucks := memory.NewTruckRepository()
	posts := memory.NewPostRepository()
	comments := memory.NewCommentRepository()
	followers := memory.NewFollowerRepository()
	feed := memory.NewFeedRepository(posts, followers)
	invitations := memory.NewInvitationRepository()
	txManager := memory.NewTransactionManager(users, invitations)

	return Storage{
		Users:       users,
		Trucks:      trucks,
		Posts:       posts,
		Comments:    comments,
		Followers:   followers,
		Feed:        feed,
		Invitations: invitations,
		TxManager:   txManager,
	}
}

// NewPostgresStorage initializes and bundles PostgreSQL-backed repository implementations using sqlc queries.
func NewPostgresStorage(pool *pgxpool.Pool, queries *db.Queries) Storage {
	return Storage{
		Users:       postgres.NewUserRepository(queries),
		Trucks:      postgres.NewTruckRepository(queries),
		Posts:       postgres.NewPostRepository(queries),
		Comments:    postgres.NewCommentRepository(queries),
		Followers:   postgres.NewFollowerRepository(queries),
		Feed:        postgres.NewFeedRepository(queries),
		Invitations: postgres.NewInvitationRepository(queries),
		TxManager:   postgres.NewTransactionManager(pool, queries),
	}
}
