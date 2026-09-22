package postgres

import (
	"context"

	"go-learning/internal/core"
	db "go-learning/internal/db/generated"
	"go-learning/internal/logic"
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
