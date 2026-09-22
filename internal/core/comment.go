package core

import "time"

// CommentAuthor contains author details associated with a comment.
type CommentAuthor struct {
	ID        int64
	FirstName string
	LastName  string
}

// Comment represents a user comment on a post.
type Comment struct {
	ID        int64
	PostID    int64
	UserID    int64
	Content   string
	CreatedAt time.Time
	Author    CommentAuthor
}
