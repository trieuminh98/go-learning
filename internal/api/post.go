package api

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/go-playground/validator/v10"

	"go-learning/internal/core"
	"go-learning/internal/logic"
)

type userContextKey string

const contextUserIDKey userContextKey = "authenticated_user_id"

// ContextWithUserID associates an authenticated user ID with the request context.
func ContextWithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, contextUserIDKey, userID)
}

// UserIDFromContext extracts the authenticated user ID from context.
// In the current development stage, it falls back to 1 (default user ID) if unauthenticated.
// TODO: Replace fallback with strict rejection once Auth Middleware is fully wired.
func UserIDFromContext(ctx context.Context) (int64, error) {
	val := ctx.Value(contextUserIDKey)
	if id, ok := val.(int64); ok && id > 0 {
		return id, nil
	}
	// Isolated temporary fallback for dev
	return 1, nil
}

// CreatePostRequest defines the writable API contract for creating a post.
type CreatePostRequest struct {
	Title   string   `json:"title" validate:"required,max=100"`
	Content string   `json:"content" validate:"required,max=1000"`
	Tags    []string `json:"tags"`
}

// PostResponse defines the readable JSON contract for returning a post.
type PostResponse struct {
	ID        int64     `json:"id"`
	UserID    int64     `json:"user_id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Tags      []string  `json:"tags"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func toPostResponse(p core.Post) PostResponse {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return PostResponse{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Content:   p.Content,
		Tags:      tags,
		CreatedAt: p.CreatedAt,
		UpdatedAt: p.UpdatedAt,
	}
}

// CommentAuthorResponse defines the public author identity for a comment.
type CommentAuthorResponse struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// CommentResponse defines the readable JSON contract for returning a comment.
type CommentResponse struct {
	ID        int64                 `json:"id"`
	PostID    int64                 `json:"post_id"`
	UserID    int64                 `json:"user_id"`
	Content   string                `json:"content"`
	CreatedAt time.Time             `json:"created_at"`
	Author    CommentAuthorResponse `json:"author"`
}

// PostDetailResponse defines the rich readable JSON contract for post detail including its comments.
type PostDetailResponse struct {
	ID        int64             `json:"id"`
	UserID    int64             `json:"user_id"`
	Title     string            `json:"title"`
	Content   string            `json:"content"`
	Tags      []string          `json:"tags"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Comments  []CommentResponse `json:"comments"`
}

func toCommentResponse(c core.Comment) CommentResponse {
	return CommentResponse{
		ID:        c.ID,
		PostID:    c.PostID,
		UserID:    c.UserID,
		Content:   c.Content,
		CreatedAt: c.CreatedAt,
		Author: CommentAuthorResponse{
			ID:        c.Author.ID,
			FirstName: c.Author.FirstName,
			LastName:  c.Author.LastName,
		},
	}
}

func toPostDetailResponse(detail core.PostDetail) PostDetailResponse {
	tags := detail.Tags
	if tags == nil {
		tags = []string{}
	}

	comments := make([]CommentResponse, 0, len(detail.Comments))
	for _, c := range detail.Comments {
		comments = append(comments, toCommentResponse(c))
	}

	return PostDetailResponse{
		ID:        detail.ID,
		UserID:    detail.UserID,
		Title:     detail.Title,
		Content:   detail.Content,
		Tags:      tags,
		CreatedAt: detail.CreatedAt,
		UpdatedAt: detail.UpdatedAt,
		Comments:  comments,
	}
}

// PostHandler coordinates HTTP requests for the posts resource.
type PostHandler struct {
	service   *logic.PostService
	validator *validator.Validate
}

// NewPostHandler creates a new PostHandler.
func NewPostHandler(service *logic.PostService, validator *validator.Validate) *PostHandler {
	return &PostHandler{
		service:   service,
		validator: validator,
	}
}

// CreatePost handles POST /v1/posts.
func (h *PostHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var input CreatePostRequest

	if err := readJSON(w, r, &input); err != nil {
		badRequest(w, r, err)
		return
	}

	if h.validator != nil {
		if err := h.validator.Struct(input); err != nil {
			badRequest(w, r, errors.New("invalid request payload"))
			return
		}
	}

	userID, err := UserIDFromContext(r.Context())
	if err != nil {
		unauthorized(w, r, "unauthorized")
		return
	}

	post, err := h.service.CreatePost(r.Context(), logic.CreatePostInput{
		UserID:  userID,
		Title:   input.Title,
		Content: input.Content,
		Tags:    input.Tags,
	})
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = writeJSON(w, http.StatusCreated, toPostResponse(post))
}

// ListPosts handles GET /v1/posts.
func (h *PostHandler) ListPosts(w http.ResponseWriter, r *http.Request) {
	posts, err := h.service.ListPosts(r.Context())
	if err != nil {
		internalServerError(w, r, err)
		return
	}

	resp := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		resp = append(resp, toPostResponse(p))
	}
	_ = writeJSON(w, http.StatusOK, resp)
}

// GetPost handles GET /v1/posts/{postID}.
func (h *PostHandler) GetPost(w http.ResponseWriter, r *http.Request) {
	postID, err := readIDParam(r, "postID")
	if err != nil {
		badRequest(w, r, err)
		return
	}

	detail, err := h.service.GetPostDetail(r.Context(), postID)
	if err != nil {
		MapDomainErrorToHTTP(w, r, err)
		return
	}

	_ = writeJSON(w, http.StatusOK, toPostDetailResponse(detail))
}
