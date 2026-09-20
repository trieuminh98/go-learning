package memory

import (
	"context"
	"sync"
	"time"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.PostRepository = (*PostRepository)(nil)

// PostRepository provides an in-memory thread-safe implementation of logic.PostRepository.
type PostRepository struct {
	mu     sync.RWMutex
	posts  []core.Post
	nextID int64
}

// NewPostRepository constructs an in-memory PostRepository.
func NewPostRepository() *PostRepository {
	return &PostRepository{
		posts:  make([]core.Post, 0),
		nextID: 1,
	}
}

// List returns all posts.
func (r *PostRepository) List(_ context.Context) ([]core.Post, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]core.Post, len(r.posts))
	copy(result, r.posts)
	return result, nil
}

// Create assigns a pseudo ID and persists the post in-memory.
func (r *PostRepository) Create(_ context.Context, p core.Post) (core.Post, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	created := core.Post{
		ID:        r.nextID,
		UserID:    p.UserID,
		Title:     p.Title,
		Content:   p.Content,
		Tags:      p.Tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	r.nextID++
	r.posts = append(r.posts, created)
	return created, nil
}
