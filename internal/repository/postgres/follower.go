package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

const (
	followTimeout   = 1 * time.Second
	unfollowTimeout = 1 * time.Second
)

// Ensure FollowerRepository satisfies logic.FollowerRepository interface at compile time.
var _ logic.FollowerRepository = (*FollowerRepository)(nil)

// FollowerRepository implements logic.FollowerRepository using sqlc-generated queries over pgx/v5.
type FollowerRepository struct {
	queries *db.Queries
}

// NewFollowerRepository constructs a PostgreSQL-backed FollowerRepository.
func NewFollowerRepository(queries *db.Queries) *FollowerRepository {
	return &FollowerRepository{
		queries: queries,
	}
}

// Follow inserts a follow relationship, mapping unique violation to core.ErrConflict
// and foreign key violation to core.ErrUserNotFound.
func (r *FollowerRepository) Follow(ctx context.Context, followerID, followedUserID int64) error {
	ctx, cancel := context.WithTimeout(ctx, followTimeout)
	defer cancel()

	err := r.queries.FollowUser(ctx, db.FollowUserParams{
		FollowerID:     followerID,
		FollowedUserID: followedUserID,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.Code {
			case "23505": // unique_violation
				return core.ErrConflict
			case "23503": // foreign_key_violation
				return core.ErrUserNotFound
			}
		}
		return err
	}

	return nil
}

// Unfollow removes a follow relationship between follower and followed user.
func (r *FollowerRepository) Unfollow(ctx context.Context, followerID, followedUserID int64) error {
	ctx, cancel := context.WithTimeout(ctx, unfollowTimeout)
	defer cancel()

	return r.queries.UnfollowUser(ctx, db.UnfollowUserParams{
		FollowerID:     followerID,
		FollowedUserID: followedUserID,
	})
}
