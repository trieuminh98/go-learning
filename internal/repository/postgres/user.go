package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"

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

func toCoreUser(id int64, firstName, lastName string, email pgtype.Text, isActive bool) core.User {
	u := core.User{
		ID:        strconv.FormatInt(id, 10),
		FirstName: firstName,
		LastName:  lastName,
		IsActive:  isActive,
	}
	if email.Valid {
		u.Email = email.String
	}
	return u
}

// List fetches all users from PostgreSQL ordered by ID ascending.
func (r *UserRepository) List(ctx context.Context) ([]core.User, error) {
	rows, err := r.queries.ListUsers(ctx)
	if err != nil {
		return nil, err
	}

	users := make([]core.User, len(rows))
	for i, row := range rows {
		users[i] = toCoreUser(row.ID, row.FirstName, row.LastName, pgtype.Text{}, false)
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

	return toCoreUser(created.ID, created.FirstName, created.LastName, pgtype.Text{}, false), nil
}

// CreateWithCredentials inserts a user with email, hashed password, and initial active status.
func (r *UserRepository) CreateWithCredentials(ctx context.Context, u core.User) (core.User, error) {
	created, err := r.queries.CreateUserWithCredentials(ctx, db.CreateUserWithCredentialsParams{
		FirstName: u.FirstName,
		LastName:  u.LastName,
		Email: pgtype.Text{
			String: u.Email,
			Valid:  u.Email != "",
		},
		PasswordHash: u.Password.Hash(),
		IsActive:     u.IsActive,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // unique_violation
			if pgErr.ConstraintName == "users_email_key" {
				return core.User{}, core.ErrEmailAlreadyExists
			}
			return core.User{}, core.ErrUserAlreadyExists
		}
		return core.User{}, err
	}


	res := toCoreUser(created.ID, created.FirstName, created.LastName, created.Email, created.IsActive)
	res.Password = u.Password
	return res, nil
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

	return toCoreUser(user.ID, user.FirstName, user.LastName, pgtype.Text{}, false), nil
}

// GetByEmail fetches a user by their email address from PostgreSQL.
func (r *UserRepository) GetByEmail(ctx context.Context, email string) (core.User, error) {
	ctx, cancel := context.WithTimeout(ctx, getUserTimeout)
	defer cancel()

	user, err := r.queries.GetUserByEmail(ctx, pgtype.Text{String: email, Valid: true})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return core.User{}, core.ErrUserNotFound
		}
		return core.User{}, err
	}

	res := toCoreUser(user.ID, user.FirstName, user.LastName, user.Email, user.IsActive)
	res.Password = core.NewPasswordFromHash(user.PasswordHash)
	return res, nil
}

// Delete removes a user record by ID. Used for compensation or account removal.
func (r *UserRepository) Delete(ctx context.Context, id int64) error {
	return r.queries.DeleteUser(ctx, id)
}
