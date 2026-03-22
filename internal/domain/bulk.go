package domain

// BulkCreateResult holds the outcome of a bulk issue creation.
type BulkCreateResult struct {
	Created []Issue  `json:"created"`
	Errors  []string `json:"errors,omitempty"`
	Total   int      `json:"total"`
	Batches int      `json:"batches"`
}
