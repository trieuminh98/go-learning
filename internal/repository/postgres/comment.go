package postgres

import (
	"context"
	"time"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
)

const (
	getCommentsTimeout   = 2 * time.Second
	createCommentTimeout = 1 * time.Second
)

var _ logic.CommentRepository = (*CommentRepository)(nil)

// CommentRepository implements logic.CommentRepository using sqlc-generated queries over pgx/v5.
type CommentRepository struct {
	queries *db.Queries
}

// NewCommentRepository constructs a PostgreSQL-backed CommentRepository.
func NewCommentRepository(queries *db.Queries) *CommentRepository {
	return &CommentRepository{
		queries: queries,
	}
}

func toCoreComment(row db.GetCommentsByPostIDRow) core.Comment {
	return core.Comment{
		ID:        row.ID,
		PostID:    row.PostID,
		UserID:    row.UserID,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		Author: core.CommentAuthor{
			ID:        row.AuthorID,
			FirstName: row.AuthorFirstName,
			LastName:  row.AuthorLastName,
		},
	}
}

// GetByPostID fetches comments for a given post ID, joined with user author details.
func (r *CommentRepository) GetByPostID(ctx context.Context, postID int64) ([]core.Comment, error) {
	ctx, cancel := context.WithTimeout(ctx, getCommentsTimeout)
	defer cancel()

	rows, err := r.queries.GetCommentsByPostID(ctx, postID)
	if err != nil {
		return nil, err
	}

	comments := make([]core.Comment, len(rows))
	for i, row := range rows {
		comments[i] = toCoreComment(row)
	}
	return comments, nil
}

// Create inserts a comment into PostgreSQL.
func (r *CommentRepository) Create(ctx context.Context, c core.Comment) (core.Comment, error) {
	ctx, cancel := context.WithTimeout(ctx, createCommentTimeout)
	defer cancel()

	row, err := r.queries.CreateComment(ctx, db.CreateCommentParams{
		PostID:  c.PostID,
		UserID:  c.UserID,
		Content: c.Content,
	})
	if err != nil {
		return core.Comment{}, err
	}

	return core.Comment{
		ID:        row.ID,
		PostID:    row.PostID,
		UserID:    row.UserID,
		Content:   row.Content,
		CreatedAt: row.CreatedAt.Time,
		Author:    c.Author,
	}, nil
}
