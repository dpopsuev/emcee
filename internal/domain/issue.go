// Package domain contains the core business objects with zero external dependencies.
package domain

import "time"

type Priority int

const (
	PriorityNone   Priority = 0
	PriorityUrgent Priority = 1
	PriorityHigh   Priority = 2
	PriorityMedium Priority = 3
	PriorityLow    Priority = 4
)

func (p Priority) String() string {
	switch p {
	case PriorityUrgent:
		return "urgent"
	case PriorityHigh:
		return "high"
	case PriorityMedium:
		return "medium"
	case PriorityLow:
		return "low"
	default:
		return "none"
	}
}

func ParsePriority(s string) Priority {
	switch s {
	case "urgent":
		return PriorityUrgent
	case "high":
		return PriorityHigh
	case "medium":
		return PriorityMedium
	case "low":
		return PriorityLow
	default:
		return PriorityNone
	}
}

type Status string

const (
	StatusBacklog    Status = "backlog"
	StatusTodo       Status = "todo"
	StatusInProgress Status = "in_progress"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
	StatusCanceled   Status = "canceled"
)

// Issue is the canonical domain object — the unified representation of a work item
// regardless of which platform it lives on.
type Issue struct {
	Ref         string    `json:"ref"`
	ID          string    `json:"id"`
	Key         string    `json:"key"`
	Title       string    `json:"title"`
	Description string    `json:"description,omitempty"`
	Status      Status    `json:"status"`
	Priority    Priority  `json:"priority"`
	Labels      []string  `json:"labels,omitempty"`
	Assignee    string    `json:"assignee,omitempty"`
	Project     string    `json:"project,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	URL         string    `json:"url,omitempty"`
}

type CreateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Status      Status   `json:"status,omitempty"`
	Priority    Priority `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Project     string   `json:"project,omitempty"`
}

type UpdateInput struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Status      *Status   `json:"status,omitempty"`
	Priority    *Priority `json:"priority,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Assignee    *string   `json:"assignee,omitempty"`
}

type ListFilter struct {
	Project  string   `json:"project,omitempty"`
	Status   Status   `json:"status,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Assignee string   `json:"assignee,omitempty"`
	Query    string   `json:"query,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}
