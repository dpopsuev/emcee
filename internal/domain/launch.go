package domain

import "time"

// Launch represents a test execution run in Report Portal.
type Launch struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Status      string           `json:"status"`
	Description string           `json:"description,omitempty"`
	Owner       string           `json:"owner,omitempty"`
	StartTime   time.Time        `json:"start_time"`
	EndTime     time.Time        `json:"end_time,omitempty"`
	Statistics  LaunchStatistics `json:"statistics"`
	URL         string           `json:"url,omitempty"`
}

// LaunchStatistics holds execution counts and defect breakdown.
type LaunchStatistics struct {
	Total   int            `json:"total"`
	Passed  int            `json:"passed"`
	Failed  int            `json:"failed"`
	Skipped int            `json:"skipped"`
	Defects map[string]int `json:"defects,omitempty"`
}

// LaunchFilter controls which launches to list.
type LaunchFilter struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// TestItem represents a single test result within a launch.
type TestItem struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Type      string `json:"type,omitempty"`
	LaunchID  string `json:"launch_id"`
	IssueType string `json:"issue_type,omitempty"`
	Comment   string `json:"comment,omitempty"`
	URL       string `json:"url,omitempty"`
}

// TestItemFilter controls which test items to list.
type TestItemFilter struct {
	Status string `json:"status,omitempty"`
	Type   string `json:"type,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

// DefectUpdate specifies a defect type change on a test item.
type DefectUpdate struct {
	TestItemID string `json:"test_item_id"`
	IssueType  string `json:"issue_type"`
	Comment    string `json:"comment,omitempty"`
}
