package memory

import (
	"context"
	"fmt"
	"sync"

	"go-learning/internal/core"
)

// FollowerRepository is an in-memory implementation of logic.FollowerRepository.
type FollowerRepository struct {
	mu        sync.RWMutex
	relations map[string]struct{}
}

// NewFollowerRepository initializes and returns a thread-safe in-memory follower repository.
func NewFollowerRepository() *FollowerRepository {
	return &FollowerRepository{
		relations: make(map[string]struct{}),
	}
}

func relationKey(followerID, followedUserID int64) string {
	return fmt.Sprintf("%d:%d", followerID, followedUserID)
}

// Follow inserts a follow relationship in-memory, returning core.ErrConflict if it already exists.
func (r *FollowerRepository) Follow(ctx context.Context, followerID, followedUserID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := relationKey(followerID, followedUserID)
	if _, exists := r.relations[key]; exists {
		return core.ErrConflict
	}

	r.relations[key] = struct{}{}
	return nil
}

// Unfollow deletes a follow relationship in-memory.
func (r *FollowerRepository) Unfollow(ctx context.Context, followerID, followedUserID int64) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	key := relationKey(followerID, followedUserID)
	delete(r.relations, key)
	return nil
}
