package postgres

import (
	"context"
	"time"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

const (
	feedTimeout = 2 * time.Second
)

// Ensure FeedRepository satisfies logic.FeedRepository interface at compile time.
var _ logic.FeedRepository = (*FeedRepository)(nil)

// FeedRepository implements logic.FeedRepository using sqlc-generated queries over pgx/v5.
type FeedRepository struct {
	queries *db.Queries
}

// NewFeedRepository constructs a PostgreSQL-backed FeedRepository.
func NewFeedRepository(queries *db.Queries) *FeedRepository {
	return &FeedRepository{
		queries: queries,
	}
}

// GetUserFeed retrieves the chronological feed for a user with bounded query timeout, optionally filtered.
func (r *FeedRepository) GetUserFeed(ctx context.Context, userID int64, filter logic.FeedFilter, limit int, offset int) ([]core.Post, error) {
	ctx, cancel := context.WithTimeout(ctx, feedTimeout)
	defer cancel()

	var tag string
	if filter.Tag != nil {
		tag = *filter.Tag
	}

	var search string
	if filter.Search != nil {
		search = *filter.Search
	}

	rows, err := r.queries.GetUserFeed(ctx, db.GetUserFeedParams{
		FollowerID: userID,
		Tag:        tag,
		Search:     search,
		Limit:      int32(limit),
		Offset:     int32(offset),
	})
	if err != nil {
		return nil, err
	}

	posts := make([]core.Post, 0, len(rows))
	for _, row := range rows {
		posts = append(posts, toCorePost(row))
	}

	return posts, nil
}
