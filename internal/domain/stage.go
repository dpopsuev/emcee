package domain

import "time"

// StagedItem is an issue creation queued for later submission.
type StagedItem struct {
	ID        string      `json:"id"`
	Backend   string      `json:"backend"`
	Input     CreateInput `json:"input"`
	Reason    string      `json:"reason,omitempty"`
	CreatedAt time.Time   `json:"created_at"`
}
