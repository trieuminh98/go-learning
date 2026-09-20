package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

var _ logic.PostRepository = (*PostRepository)(nil)

// PostRepository implements logic.PostRepository using sqlc-generated queries over pgx/v5.
type PostRepository struct {
	queries *db.Queries
}

// NewPostRepository constructs a PostgreSQL-backed PostRepository.
func NewPostRepository(queries *db.Queries) *PostRepository {
	return &PostRepository{
		queries: queries,
	}
}

func toCorePost(p db.Post) core.Post {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return core.Post{
		ID:        p.ID,
		UserID:    p.UserID,
		Title:     p.Title,
		Content:   p.Content,
		Tags:      tags,
		CreatedAt: p.CreatedAt.Time,
		UpdatedAt: p.UpdatedAt.Time,
	}
}

// List fetches all posts from PostgreSQL.
func (r *PostRepository) List(ctx context.Context) ([]core.Post, error) {
	rows, err := r.queries.ListPosts(ctx)
	if err != nil {
		return nil, err
	}

	posts := make([]core.Post, len(rows))
	for i, row := range rows {
		posts[i] = toCorePost(row)
	}
	return posts, nil
}

// Create inserts a post into PostgreSQL, mapping foreign key violations to core.ErrUserNotFound.
func (r *PostRepository) Create(ctx context.Context, p core.Post) (core.Post, error) {
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}

	created, err := r.queries.CreatePost(ctx, db.CreatePostParams{
		UserID:  p.UserID,
		Title:   p.Title,
		Content: p.Content,
		Tags:    tags,
	})
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" { // foreign_key_violation
			return core.Post{}, core.ErrUserNotFound
		}
		return core.Post{}, err
	}

	return toCorePost(created), nil
}
