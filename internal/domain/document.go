package domain

import "time"

// Document is the canonical domain object for rich-text documents.
type Document struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content,omitempty"`
	ProjectID string    `json:"project_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	URL       string    `json:"url,omitempty"`
}

type DocumentCreateInput struct {
	Title     string `json:"title"`
	Content   string `json:"content,omitempty"`
	ProjectID string `json:"project_id,omitempty"`
}

type DocumentListFilter struct {
	Limit int `json:"limit,omitempty"`
}
