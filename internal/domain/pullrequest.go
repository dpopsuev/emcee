package domain

import "time"

// PullRequest represents a pull request (GitHub) or merge request (GitLab).
type PullRequest struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Author   string    `json:"author"`
	State    string    `json:"state"`
	MergedAt time.Time `json:"merged_at,omitempty"`
	URL      string    `json:"url"`
	Repo     string    `json:"repo,omitempty"`
}

// PRFilter controls which pull requests to list.
type PRFilter struct {
	Author       string `json:"author,omitempty"`
	State        string `json:"state,omitempty"`
	MergedAfter  string `json:"merged_after,omitempty"`
	MergedBefore string `json:"merged_before,omitempty"`
	Limit        int    `json:"limit,omitempty"`
}
