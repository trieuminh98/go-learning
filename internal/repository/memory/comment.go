package memory

import (
	"context"
	"sync"
	"time"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

var _ logic.CommentRepository = (*CommentRepository)(nil)

// CommentRepository provides an in-memory thread-safe implementation of logic.CommentRepository.
type CommentRepository struct {
	mu       sync.RWMutex
	comments []core.Comment
	nextID   int64
}

// NewCommentRepository constructs an in-memory CommentRepository.
func NewCommentRepository() *CommentRepository {
	return &CommentRepository{
		comments: make([]core.Comment, 0),
		nextID:   1,
	}
}

// GetByPostID fetches comments for a given post ID from the in-memory store.
func (r *CommentRepository) GetByPostID(_ context.Context, postID int64) ([]core.Comment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []core.Comment
	for _, c := range r.comments {
		if c.PostID == postID {
			result = append(result, c)
		}
	}
	if result == nil {
		result = []core.Comment{}
	}
	return result, nil
}

// Create stores a new comment in-memory.
func (r *CommentRepository) Create(_ context.Context, c core.Comment) (core.Comment, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	created := core.Comment{
		ID:        r.nextID,
		PostID:    c.PostID,
		UserID:    c.UserID,
		Content:   c.Content,
		CreatedAt: time.Now().UTC(),
		Author:    c.Author,
	}
	r.nextID++
	r.comments = append(r.comments, created)
	return created, nil
}
