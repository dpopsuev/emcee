package domain

import "time"

// WorkflowRun represents a GitHub Actions workflow run.
type WorkflowRun struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	Branch     string    `json:"branch,omitempty"`
	Event      string    `json:"event,omitempty"`
	URL        string    `json:"url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// WorkflowRunFilter constrains workflow run queries.
type WorkflowRunFilter struct {
	Status string `json:"status,omitempty"` // queued, in_progress, completed
	Branch string `json:"branch,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// WorkflowJob represents a job within a workflow run.
type WorkflowJob struct {
	ID         int64     `json:"id"`
	RunID      int64     `json:"run_id"`
	Name       string    `json:"name"`
	Status     string    `json:"status"`
	Conclusion string    `json:"conclusion,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	Duration   int64     `json:"duration_ms,omitempty"`
	Steps      []JobStep `json:"steps,omitempty"`
}

// JobStep represents a single step within a workflow job.
type JobStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion,omitempty"`
	Number     int    `json:"number"`
}
