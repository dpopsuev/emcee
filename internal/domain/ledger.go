package domain

import "time"

// ArtifactRecord is a unified snapshot of any artifact seen by the system.
// The ledger stores these for cross-cutting queries across backends.
type ArtifactRecord struct {
	Ref        string    `json:"ref"`
	Backend    string    `json:"backend"`
	Type       string    `json:"type"` // issue, pr, build, launch, test_item
	Title      string    `json:"title"`
	URL        string    `json:"url,omitempty"`
	Status     string    `json:"status,omitempty"`
	Labels     []string  `json:"labels,omitempty"`
	Components []string  `json:"components,omitempty"`
	Text       string    `json:"text,omitempty"` // description + comments
	SeenAt     time.Time `json:"seen_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// LedgerFilter constrains ledger queries.
type LedgerFilter struct {
	Backend   string `json:"backend,omitempty"`
	Type      string `json:"type,omitempty"`
	Component string `json:"component,omitempty"`
	Status    string `json:"status,omitempty"`
	Limit     int    `json:"limit,omitempty"`
}

// LedgerStats summarizes the ledger contents.
type LedgerStats struct {
	Total     int            `json:"total"`
	ByBackend map[string]int `json:"by_backend"`
}
