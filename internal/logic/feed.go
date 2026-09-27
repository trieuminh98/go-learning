package logic

import (
	"context"

	"go-learning/internal/core"
)

const (
	defaultFeedLimit = 20
	maxFeedLimit     = 100
)

// FeedFilter specifies optional criteria to narrow down feed posts.
type FeedFilter struct {
	Tag    *string
	Search *string
}

// FeedService coordinates business operations around user feeds.
type FeedService struct {
	repo FeedRepository
}

// NewFeedService constructs a new FeedService.
func NewFeedService(repo FeedRepository) *FeedService {
	return &FeedService{repo: repo}
}

// GetFeed retrieves a bounded chronological feed for the specified actor, optionally filtered.
// Enforces business invariants: default limit of 20, strict upper ceiling of 100,
// non-negative offset, and ensures a non-nil slice is returned on empty feeds.
func (s *FeedService) GetFeed(ctx context.Context, actorID int64, filter FeedFilter, limit int, offset int) ([]core.Post, error) {
	if limit <= 0 {
		limit = defaultFeedLimit
	}
	if limit > maxFeedLimit {
		limit = maxFeedLimit
	}
	if offset < 0 {
		offset = 0
	}

	posts, err := s.repo.GetUserFeed(ctx, actorID, filter, limit, offset)
	if err != nil {
		return nil, err
	}

	if posts == nil {
		posts = []core.Post{}
	}

	return posts, nil
}
