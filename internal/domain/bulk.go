package domain

// BulkCreateResult holds the outcome of a bulk issue creation.
type BulkCreateResult struct {
	Created []Issue  `json:"created"`
	Errors  []string `json:"errors,omitempty"`
	Total   int      `json:"total"`
	Batches int      `json:"batches"`
}

// BulkUpdateInput pairs a ref with the fields to update.
type BulkUpdateInput struct {
	Ref         string   `json:"ref"`
	Title       *string  `json:"title,omitempty"`
	Description *string  `json:"description,omitempty"`
	Status      *Status  `json:"status,omitempty"`
	Priority    *Priority `json:"priority,omitempty"`
}

// BulkUpdateResult holds the outcome of a bulk issue update.
type BulkUpdateResult struct {
	Updated []Issue  `json:"updated"`
	Errors  []string `json:"errors,omitempty"`
	Total   int      `json:"total"`
}
