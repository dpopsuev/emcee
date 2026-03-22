package domain

import "time"

// Project groups issues and documents into an epic-level container.
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Status      string    `json:"status,omitempty"`
	URL         string    `json:"url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type ProjectCreateInput struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	TeamIDs     []string `json:"team_ids,omitempty"`
}

type ProjectUpdateInput struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type ProjectListFilter struct {
	Limit int `json:"limit,omitempty"`
}
