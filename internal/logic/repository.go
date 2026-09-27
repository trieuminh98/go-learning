package logic

import (
	"context"

	"go-learning/internal/core"
)

// UserRepository defines the persistence contract required by the user domain logic.
// Kept small, context-aware, and free of database-specific or transport-specific types.
type UserRepository interface {
	List(ctx context.Context) ([]core.User, error)
	GetByID(ctx context.Context, id int64) (core.User, error)
	Create(ctx context.Context, user core.User) (core.User, error)
}

// TruckRepository defines the persistence contract required by the truck domain logic.
type TruckRepository interface {
	List(ctx context.Context) ([]core.Truck, error)
	GetByID(ctx context.Context, id string) (core.Truck, error)
	Create(ctx context.Context, truck core.Truck) error
}

// PostRepository defines the persistence contract required by the post domain logic.
type PostRepository interface {
	List(ctx context.Context) ([]core.Post, error)
	Create(ctx context.Context, post core.Post) (core.Post, error)
	GetByID(ctx context.Context, id int64) (core.Post, error)
	Update(ctx context.Context, post core.Post) (core.Post, error)
	Delete(ctx context.Context, id int64) error
}

// CommentRepository defines the persistence contract required for comment operations.
type CommentRepository interface {
	GetByPostID(ctx context.Context, postID int64) ([]core.Comment, error)
	Create(ctx context.Context, comment core.Comment) (core.Comment, error)
}

// FollowerRepository defines the persistence contract required for user follow relationships.
type FollowerRepository interface {
	Follow(ctx context.Context, followerID int64, followedUserID int64) error
	Unfollow(ctx context.Context, followerID int64, followedUserID int64) error
}

// FeedRepository defines the persistence contract required for retrieving user feeds.
type FeedRepository interface {
	GetUserFeed(ctx context.Context, userID int64, limit int) ([]core.Post, error)
}
