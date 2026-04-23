// Package domain contains the core business objects with zero external dependencies.
package domain

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// ErrInvalidPriority indicates an unparseable priority value.
var ErrInvalidPriority = errors.New("priority must be int or string")

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

// UnmarshalJSON accepts both int (0-4) and string ("urgent", "high", etc.).
func (p *Priority) UnmarshalJSON(data []byte) error {
	var n int
	if err := json.Unmarshal(data, &n); err == nil {
		*p = Priority(n)
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*p = ParsePriority(s)
		return nil
	}
	return fmt.Errorf("%w, got %s", ErrInvalidPriority, string(data))
}

// MarshalJSON serializes priority as int for Linear API compatibility.
func (p Priority) MarshalJSON() ([]byte, error) {
	return json.Marshal(int(p))
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
	Ref           string         `json:"ref"`
	ID            string         `json:"id"`
	Key           string         `json:"key"`
	Title         string         `json:"title"`
	Description   string         `json:"description,omitempty"`
	Status        Status         `json:"status"`
	Priority      Priority       `json:"priority"`
	Labels        []string       `json:"labels,omitempty"`
	Assignee      string         `json:"assignee,omitempty"`
	Project       string         `json:"project,omitempty"`
	IssueType     string         `json:"issue_type,omitempty"`
	Resolution    string         `json:"resolution,omitempty"`
	FixVersions   []string       `json:"fix_versions,omitempty"`
	Components    []string       `json:"components,omitempty"`
	Comments      []Comment      `json:"comments,omitempty"`
	IssueLinks    []IssueLink    `json:"issue_links,omitempty"`
	ExternalLinks []ExternalLink `json:"external_links,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	URL           string         `json:"url,omitempty"`
}

type CreateInput struct {
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Status      Status   `json:"status,omitempty"`
	Priority    Priority `json:"priority,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	Assignee    string   `json:"assignee,omitempty"`
	Project     string   `json:"project,omitempty"`
	ParentID    string   `json:"parent_id,omitempty"`
	ProjectID   string   `json:"project_id,omitempty"`
	IssueType   string   `json:"issue_type,omitempty"`
	Components  []string `json:"components,omitempty"`
	FixVersions []string `json:"fix_versions,omitempty"`
	Versions    []string `json:"versions,omitempty"`
}

type UpdateInput struct {
	Title       *string   `json:"title,omitempty"`
	Description *string   `json:"description,omitempty"`
	Status      *Status   `json:"status,omitempty"`
	Priority    *Priority `json:"priority,omitempty"`
	Labels      []string  `json:"labels,omitempty"`
	Assignee    *string   `json:"assignee,omitempty"`
	Components  []string  `json:"components,omitempty"`
	FixVersions []string  `json:"fix_versions,omitempty"`
	Resolution  *string   `json:"resolution,omitempty"`
}

// IssueLink represents a link between two Jira issues.
type IssueLink struct {
	Type         string `json:"type"`
	Direction    string `json:"direction"`
	TargetRef    string `json:"target_ref"`
	TargetKey    string `json:"target_key"`
	TargetTitle  string `json:"target_title"`
	TargetStatus string `json:"target_status,omitempty"`
}

// ExternalLink represents a remote link (PR, commit, Confluence page) on an issue.
type ExternalLink struct {
	Title string `json:"title"`
	URL   string `json:"url"`
	Type  string `json:"type,omitempty"`
}

// IssueLinkInput is the input for creating an issue link.
type IssueLinkInput struct {
	Type       string `json:"type"`
	InwardKey  string `json:"inward_key"`
	OutwardKey string `json:"outward_key"`
}

type ListFilter struct {
	Project  string   `json:"project,omitempty"`
	Status   Status   `json:"status,omitempty"`
	Labels   []string `json:"labels,omitempty"`
	Assignee string   `json:"assignee,omitempty"`
	Query    string   `json:"query,omitempty"`
	Limit    int      `json:"limit,omitempty"`
}
