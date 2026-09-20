package logic

import (
	"context"
	"strings"

	"go-learning/internal/core"
)

// CreatePostInput defines the application use-case parameters for creating a post.
type CreatePostInput struct {
	UserID  int64
	Title   string
	Content string
	Tags    []string
}

// PostService coordinates business operations around posts.
type PostService struct {
	repo PostRepository
}

// NewPostService constructs a new PostService.
func NewPostService(repo PostRepository) *PostService {
	return &PostService{repo: repo}
}

// CreatePost validates application rules, creates the domain entity, and persists it.
func (s *PostService) CreatePost(ctx context.Context, input CreatePostInput) (core.Post, error) {
	trimmedTitle := strings.TrimSpace(input.Title)
	if trimmedTitle == "" {
		return core.Post{}, core.ErrInvalidTitle
	}

	trimmedContent := strings.TrimSpace(input.Content)
	if trimmedContent == "" {
		return core.Post{}, core.ErrInvalidContent
	}

	tags := input.Tags
	if tags == nil {
		tags = []string{}
	}

	post := core.Post{
		UserID:  input.UserID,
		Title:   trimmedTitle,
		Content: trimmedContent,
		Tags:    tags,
	}

	return s.repo.Create(ctx, post)
}

// ListPosts returns all posts.
func (s *PostService) ListPosts(ctx context.Context) ([]core.Post, error) {
	return s.repo.List(ctx)
}
