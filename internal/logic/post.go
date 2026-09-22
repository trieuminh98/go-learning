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
	repo        PostRepository
	commentRepo CommentRepository
}

// NewPostService constructs a new PostService.
func NewPostService(repo PostRepository, commentRepo CommentRepository) *PostService {
	return &PostService{
		repo:        repo,
		commentRepo: commentRepo,
	}
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

// GetPost retrieves a single post by its ID without loading comments.
func (s *PostService) GetPost(ctx context.Context, id int64) (core.Post, error) {
	return s.repo.GetByID(ctx, id)
}

// GetPostDetail retrieves a post along with its comments for the detail view.
func (s *PostService) GetPostDetail(ctx context.Context, id int64) (core.PostDetail, error) {
	post, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return core.PostDetail{}, err
	}

	comments, err := s.commentRepo.GetByPostID(ctx, id)
	if err != nil {
		return core.PostDetail{}, err
	}
	if comments == nil {
		comments = []core.Comment{}
	}

	return core.PostDetail{
		Post:     post,
		Comments: comments,
	}, nil
}

// UpdatePost validates domain rules and updates the existing post entity.
func (s *PostService) UpdatePost(ctx context.Context, post core.Post) (core.Post, error) {
	trimmedTitle := strings.TrimSpace(post.Title)
	if trimmedTitle == "" {
		return core.Post{}, core.ErrInvalidTitle
	}

	trimmedContent := strings.TrimSpace(post.Content)
	if trimmedContent == "" {
		return core.Post{}, core.ErrInvalidContent
	}

	post.Title = trimmedTitle
	post.Content = trimmedContent
	if post.Tags == nil {
		post.Tags = []string{}
	}

	return s.repo.Update(ctx, post)
}

// DeletePost removes a post by ID without requiring a prior load.
func (s *PostService) DeletePost(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}
