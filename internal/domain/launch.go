package domain

import "time"

// Launch represents a test execution run in Report Portal.
type Launch struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Status      string            `json:"status"`
	Description string            `json:"description,omitempty"`
	Owner       string            `json:"owner,omitempty"`
	StartTime   time.Time         `json:"start_time"`
	EndTime     time.Time         `json:"end_time,omitempty"`
	Statistics  LaunchStatistics  `json:"statistics"`
	Attributes  []LaunchAttribute `json:"attributes,omitempty"`
	URL         string            `json:"url,omitempty"`
}

// LaunchAttribute is a key-value tag on a launch — CI systems use these to embed build_url, job name, etc.
type LaunchAttribute struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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
	Name        string            `json:"name,omitempty"`
	Status      string            `json:"status,omitempty"`
	StartAfter  time.Time         `json:"start_after,omitempty"`  // list launches that started after this time
	StartBefore time.Time         `json:"start_before,omitempty"` // list launches that started before this time
	Attributes  map[string]string `json:"attributes,omitempty"`   // exact attribute key=value filters (e.g. "ci-lane"="telco-ft-ran-ptp")
	Limit       int               `json:"limit,omitempty"`
	Page        int               `json:"page,omitempty"` // 0-based page number for pagination
}

// TestItem represents a single test result within a launch.
type TestItem struct {
	ID                   string                `json:"id"`
	Name                 string                `json:"name"`
	Status               string                `json:"status"`
	Type                 string                `json:"type,omitempty"`
	ParentID             string                `json:"parent_id,omitempty"`
	LaunchID             string                `json:"launch_id"`
	IssueType            string                `json:"issue_type,omitempty"`
	Comment              string                `json:"comment,omitempty"`
	FailureMessage       string                `json:"failure_message,omitempty"`
	ExternalSystemIssues []ExternalSystemIssue `json:"external_system_issues,omitempty"`
	URL                  string                `json:"url,omitempty"`
}

// TestItemFilter controls which test items to list or search.
type TestItemFilter struct {
	Name        string `json:"name,omitempty"` // substring filter on test item name
	Status      string `json:"status,omitempty"`
	IssueType   string `json:"issue_type,omitempty"` // e.g. "ti001", "pb001", "ab001"
	Type        string `json:"type,omitempty"`
	Limit       int    `json:"limit,omitempty"`
	Page        int    `json:"page,omitempty"`         // 0-based page number for pagination
	IncludeLogs bool   `json:"include_logs,omitempty"` // fetch failure_message for FAILED items

	// Cross-launch search fields. The application layer resolves LaunchName/Since/Before
	// into LaunchIDs before calling the repository.
	LaunchIDs        []string          `json:"launch_ids,omitempty"`        // resolved by application layer
	LaunchName       string            `json:"launch_name,omitempty"`       // launch name substring filter
	LaunchAttributes map[string]string `json:"launch_attributes,omitempty"` // exact attribute filters (e.g. "ci-lane"="telco-ft-ran-ptp")
	Since            time.Time         `json:"since,omitempty"`             // launch start time lower bound
	Before           time.Time         `json:"before,omitempty"`            // launch start time upper bound
}

// ExternalSystemIssue links a test item defect to an external bug tracker (e.g. Jira).
type ExternalSystemIssue struct {
	TicketID   string `json:"ticket_id"`
	BtsURL     string `json:"bts_url"`
	BtsProject string `json:"bts_project"`
	URL        string `json:"url,omitempty"`
}

// Dashboard represents a Report Portal dashboard.
type Dashboard struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Widgets     []Widget `json:"widgets,omitempty"`
}

// Widget represents a widget on a Report Portal dashboard.
type Widget struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// DashboardCreateInput is the input for creating a dashboard.
type DashboardCreateInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// WidgetAddInput is the input for adding a widget to a dashboard.
type WidgetAddInput struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Width  int    `json:"width,omitempty"`
	Height int    `json:"height,omitempty"`
}

// DefectUpdate specifies a defect type change on a test item.
type DefectUpdate struct {
	TestItemID           string                `json:"test_item_id"`
	IssueType            string                `json:"issue_type"`
	Comment              string                `json:"comment,omitempty"`
	ExternalSystemIssues []ExternalSystemIssue `json:"external_system_issues,omitempty"`
}
