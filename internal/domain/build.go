package domain

import "time"

// BuildResult represents the outcome of a Jenkins build.
type BuildResult string

const (
	BuildSuccess  BuildResult = "SUCCESS"
	BuildFailure  BuildResult = "FAILURE"
	BuildUnstable BuildResult = "UNSTABLE"
	BuildAborted  BuildResult = "ABORTED"
	BuildNotBuilt BuildResult = "NOT_BUILT"
)

// Job represents a Jenkins job (project/pipeline).
type Job struct {
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	Color     string `json:"color,omitempty"`
	Buildable bool   `json:"buildable"`
	InQueue   bool   `json:"in_queue"`
}

// Build represents a single execution of a Jenkins job.
type Build struct {
	Number    int64       `json:"number"`
	Result    BuildResult `json:"result"`
	Building  bool        `json:"building"`
	Duration  int64       `json:"duration_ms"`
	Timestamp time.Time   `json:"timestamp"`
	URL       string      `json:"url,omitempty"`
}

// TestResult holds aggregated test execution results for a build.
type TestResult struct {
	Passed   int         `json:"passed"`
	Failed   int         `json:"failed"`
	Skipped  int         `json:"skipped"`
	Total    int         `json:"total"`
	Duration float64     `json:"duration_s"`
	Suites   []TestSuite `json:"suites,omitempty"`
}

// TestSuite represents a single test suite within a build's results.
type TestSuite struct {
	Name     string     `json:"name"`
	Duration float64    `json:"duration_s"`
	Cases    []TestCase `json:"cases,omitempty"`
}

// TestCase represents a single test within a suite.
type TestCase struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// QueueItem represents a build waiting in the Jenkins queue.
type QueueItem struct {
	ID        int64  `json:"id"`
	Why       string `json:"why,omitempty"`
	Blocked   bool   `json:"blocked"`
	Buildable bool   `json:"buildable"`
	TaskName  string `json:"task_name"`
}

// JobParameter describes a build parameter definition on a Jenkins job.
type JobParameter struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	DefaultValue string `json:"default_value,omitempty"`
	Description  string `json:"description,omitempty"`
}

// BuildSummary is a lightweight build reference (number + URL).
type BuildSummary struct {
	Number int64  `json:"number"`
	URL    string `json:"url"`
}

// JobFilter controls which jobs to list.
type JobFilter struct {
	Limit int `json:"limit,omitempty"`
}
