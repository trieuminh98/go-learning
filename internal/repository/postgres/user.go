package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

const (
	getUserTimeout = 500 * time.Millisecond
)

// Ensure UserRepository satisfies logic.UserRepository interface at compile time.
var _ logic.UserRepository = (*UserRepository)(nil)

// UserRepository implements logic.UserRepository using sqlc-generated queries over pgx/v5.
type UserRepository struct {
	queries *db.Queries
}

// NewUserRepository constructs a PostgreSQL-backed UserRepository.
func NewUserRepository(queries *db.Queries) *UserRepository {
	return &UserRepository{
		queries: queries,
	}
}

func toCoreUser(u db.User) core.User {
	return core.User{
		ID:        strconv.FormatInt(u.ID, 10),
		FirstName: u.FirstName,
		LastName:  u.LastName,
	}
}

// List fetches all users from PostgreSQL ordered by ID ascending.
func (r *UserRepository) List(ctx context.Context) ([]core.User, error) {
	rows, err := r.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	users := make([]core.User, len(rows))
	for i, row := range rows {
		users[i] = toCoreUser(row)
	}
	return users, nil
}

// Create inserts a user into PostgreSQL and maps unique constraint violations to domain errors.
func (r *UserRepository) Create(ctx context.Context, u core.User) (core.User, error) {
	created, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		FirstName: u.FirstName,
		LastName:  u.LastName,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			return core.User{}, core.ErrUserAlreadyExists
		}
		return core.User{}, err
	}

	return toCoreUser(created), nil
}

// GetByID fetches a user by their ID from PostgreSQL, mapping pgx.ErrNoRows to core.ErrUserNotFound.
func (r *UserRepository) GetByID(ctx context.Context, id int64) (core.User, error) {
	ctx, cancel := context.WithTimeout(ctx, getUserTimeout)
	defer cancel()

	user, err := r.queries.GetUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, core.ErrUserNotFound
		}
		return core.User{}, err
	}

	return toCoreUser(user), nil
}
