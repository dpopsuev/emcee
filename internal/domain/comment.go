package domain

import "time"

// Comment is a single comment on an issue.
type Comment struct {
	ID        string    `json:"id"`
	Body      string    `json:"body"`
	Author    string    `json:"author,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// CommentCreateInput holds the fields needed to add a comment.
type CommentCreateInput struct {
	Body string `json:"body"`
}
