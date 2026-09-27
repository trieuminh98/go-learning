package memory

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.FeedRepository = (*FeedRepository)(nil)

// FeedRepository is an in-memory implementation of logic.FeedRepository.
type FeedRepository struct {
	mu           sync.RWMutex
	postRepo     *PostRepository
	followerRepo *FollowerRepository
}

// NewFeedRepository initializes a FeedRepository backed by in-memory post and follower repositories.
func NewFeedRepository(postRepo *PostRepository, followerRepo *FollowerRepository) *FeedRepository {
	return &FeedRepository{
		postRepo:     postRepo,
		followerRepo: followerRepo,
	}
}

// GetUserFeed returns chronological posts from users followed by userID, bounded by limit and offset.
func (r *FeedRepository) GetUserFeed(ctx context.Context, userID int64, limit int, offset int) ([]core.Post, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// 1. Identify all users followed by userID
	r.followerRepo.mu.RLock()
	followed := make(map[int64]bool)
	for key := range r.followerRepo.relations {
		var fID, targetID int64
		if _, err := fmt.Sscanf(key, "%d:%d", &fID, &targetID); err == nil {
			if fID == userID {
				followed[targetID] = true
			}
		}
	}
	r.followerRepo.mu.RUnlock()

	if len(followed) == 0 {
		return []core.Post{}, nil
	}

	// 2. Filter posts authored by followed users
	r.postRepo.mu.RLock()
	var feedPosts []core.Post
	for _, p := range r.postRepo.posts {
		if followed[p.UserID] {
			feedPosts = append(feedPosts, p)
		}
	}
	r.postRepo.mu.RUnlock()

	// 3. Sort posts deterministic newest first: created_at DESC, id DESC
	sort.Slice(feedPosts, func(i, j int) bool {
		if feedPosts[i].CreatedAt.Equal(feedPosts[j].CreatedAt) {
			return feedPosts[i].ID > feedPosts[j].ID
		}
		return feedPosts[i].CreatedAt.After(feedPosts[j].CreatedAt)
	})

	// 4. Apply offset
	if offset < 0 {
		offset = 0
	}
	if offset >= len(feedPosts) {
		return []core.Post{}, nil
	}
	feedPosts = feedPosts[offset:]

	// 5. Bound results by limit
	if limit > 0 && len(feedPosts) > limit {
		feedPosts = feedPosts[:limit]
	}

	if feedPosts == nil {
		feedPosts = []core.Post{}
	}

	return feedPosts, nil
}
