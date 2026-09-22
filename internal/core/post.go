package core

import "time"

// Post represents a published user article.
type Post struct {
	ID        int64
	UserID    int64
	Title     string
	Content   string
	Tags      []string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PostDetail represents a rich post aggregate with its associated comments for the detail view.
type PostDetail struct {
	Post
	Comments []Comment
}
