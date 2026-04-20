package domain

import "time"

// CIPipeline represents a GitLab CI pipeline.
type CIPipeline struct {
	ID        int64     `json:"id"`
	Status    string    `json:"status"`
	Ref       string    `json:"ref"`
	SHA       string    `json:"sha,omitempty"`
	Source    string    `json:"source,omitempty"`
	URL       string    `json:"url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	Duration  int64     `json:"duration_s,omitempty"`
}

// CIPipelineFilter constrains pipeline queries.
type CIPipelineFilter struct {
	Status string `json:"status,omitempty"` // running, pending, success, failed, canceled
	Ref    string `json:"ref,omitempty"`    // branch or tag
	Limit  int    `json:"limit,omitempty"`
}

// CIJob represents a job within a GitLab CI pipeline.
type CIJob struct {
	ID         int64     `json:"id"`
	PipelineID int64     `json:"pipeline_id"`
	Name       string    `json:"name"`
	Stage      string    `json:"stage"`
	Status     string    `json:"status"`
	URL        string    `json:"url,omitempty"`
	StartedAt  time.Time `json:"started_at"`
	Duration   float64   `json:"duration_s,omitempty"`
}
