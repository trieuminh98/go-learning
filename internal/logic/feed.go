package logic

import (
	"context"

	"go-learning/internal/core"
)

const (
	defaultFeedLimit = 20
	maxFeedLimit     = 100
)

// FeedService coordinates business operations around user feeds.
type FeedService struct {
	repo FeedRepository
}

// NewFeedService constructs a new FeedService.
func NewFeedService(repo FeedRepository) *FeedService {
	return &FeedService{repo: repo}
}

// GetFeed retrieves a bounded chronological feed for the specified actor.
// Enforces business invariants: default limit of 20, strict upper ceiling of 100,
// and ensures a non-nil slice is returned on empty feeds.
func (s *FeedService) GetFeed(ctx context.Context, actorID int64, limit int) ([]core.Post, error) {
	if limit <= 0 {
		limit = defaultFeedLimit
	}
	if limit > maxFeedLimit {
		limit = maxFeedLimit
	}

	posts, err := s.repo.GetUserFeed(ctx, actorID, limit)
	if err != nil {
		return nil, err
	}

	if posts == nil {
		posts = []core.Post{}
	}

	return posts, nil
}
