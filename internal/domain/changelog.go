package domain

import "time"

// ChangelogEntry is one atomic change event on an issue —
// a single author/timestamp with one or more field mutations.
type ChangelogEntry struct {
	ID      string          `json:"id"`
	Author  string          `json:"author"`
	Created time.Time       `json:"created"`
	Items   []ChangelogItem `json:"items"`
}

// ChangelogItem is the before/after state of one field within a ChangelogEntry.
type ChangelogItem struct {
	Field     string `json:"field"`      // human name, e.g. "Sprint"
	FieldID   string `json:"field_id"`   // Jira fieldId, e.g. "customfield_10020"
	FromValue string `json:"from_value"` // previous display value (empty if created)
	ToValue   string `json:"to_value"`   // new display value (empty if cleared)
}
