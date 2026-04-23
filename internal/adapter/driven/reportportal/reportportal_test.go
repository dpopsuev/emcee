package reportportal

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// Compile-time interface assertions.
var (
	_ driven.IssueRepository  = (*Repository)(nil)
	_ driven.LaunchRepository = (*Repository)(nil)
)

func TestNew(t *testing.T) {
	repo, err := New(BackendName, "https://rp.example.com", "project", "token")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if repo.Name() != BackendName {
		t.Errorf("Name() = %q, want %q", repo.Name(), BackendName)
	}
}

func TestNewTrimsTrailingSlash(t *testing.T) {
	repo, err := New(BackendName, "https://rp.example.com/", "project", "token")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if repo.baseURL != "https://rp.example.com" {
		t.Errorf("baseURL = %q, want trailing slash removed", repo.baseURL)
	}
}

func TestIssueRepositoryStubsReturnError(t *testing.T) {
	repo, _ := New(BackendName, "https://rp.example.com", "project", "token")

	ctx := context.Background()
	tests := []struct {
		name string
		fn   func() error
	}{
		{"List", func() error { _, err := repo.List(ctx, domain.ListFilter{}); return err }},
		{"Get", func() error { _, err := repo.Get(ctx, "1"); return err }},
		{"Create", func() error { _, err := repo.Create(ctx, domain.CreateInput{}); return err }},
		{"Update", func() error { _, err := repo.Update(ctx, "1", domain.UpdateInput{}); return err }},
		{"Search", func() error { _, err := repo.Search(ctx, "q", 10); return err }},
		{"ListChildren", func() error { _, err := repo.ListChildren(ctx, "1"); return err }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.fn()
			if !errors.Is(err, ErrNotIssueBackend) {
				t.Errorf("got %v, want ErrNotIssueBackend", err)
			}
		})
	}
}

func TestParseRPTimestamp_EpochMillis(t *testing.T) {
	// 1711400000000 = 2024-03-25T22:13:20Z
	raw := json.RawMessage(`1711400000000`)
	got := parseRPTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for epoch millis")
	}
	want := time.UnixMilli(1711400000000)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseRPTimestamp_ISO8601(t *testing.T) {
	raw := json.RawMessage(`"2024-03-25T22:13:20Z"`)
	got := parseRPTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for ISO 8601 string")
	}
	want, _ := time.Parse(time.RFC3339, "2024-03-25T22:13:20Z")
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseRPTimestamp_ISO8601WithMillis(t *testing.T) {
	raw := json.RawMessage(`"2024-03-25T22:13:20.123Z"`)
	got := parseRPTimestamp(raw)
	if got.IsZero() {
		t.Fatal("expected non-zero time for ISO 8601 with millis")
	}
}

func TestParseRPTimestamp_Null(t *testing.T) {
	got := parseRPTimestamp(json.RawMessage(`null`))
	if !got.IsZero() {
		t.Errorf("expected zero time for null, got %v", got)
	}
}

func TestParseRPTimestamp_Empty(t *testing.T) {
	got := parseRPTimestamp(json.RawMessage(``))
	if !got.IsZero() {
		t.Errorf("expected zero time for empty, got %v", got)
	}
}

func TestParseRPTimestamp_InvalidString(t *testing.T) {
	got := parseRPTimestamp(json.RawMessage(`"not-a-date"`))
	if !got.IsZero() {
		t.Errorf("expected zero time for invalid string, got %v", got)
	}
}

func TestRpLaunchToDomain(t *testing.T) {
	l := &rpLaunch{
		ID:          42,
		Name:        "Smoke Test",
		Status:      "PASSED",
		Description: "nightly run",
		Owner:       "ci-bot",
	}
	l.Statistics.Executions.Total = 100
	l.Statistics.Executions.Passed = 95
	l.Statistics.Executions.Failed = 3
	l.Statistics.Executions.Skipped = 2
	l.Statistics.Defects = map[string]map[string]int{
		"product_bug":    {"total": 2},
		"automation_bug": {"total": 1},
	}

	got := l.toDomain("https://rp.example.com", "myproject")
	if got.ID != "42" {
		t.Errorf("ID = %q, want %q", got.ID, "42")
	}
	if got.Name != "Smoke Test" {
		t.Errorf("Name = %q, want %q", got.Name, "Smoke Test")
	}
	if got.Statistics.Total != 100 {
		t.Errorf("Statistics.Total = %d, want 100", got.Statistics.Total)
	}
	if got.Statistics.Defects["product_bug"] != 2 {
		t.Errorf("Defects[product_bug] = %d, want 2", got.Statistics.Defects["product_bug"])
	}
	wantURL := "https://rp.example.com/ui/#myproject/launches/all/42"
	if got.URL != wantURL {
		t.Errorf("URL = %q, want %q", got.URL, wantURL)
	}
}

func TestRpTestItemToDomain(t *testing.T) {
	ti := &rpTestItem{
		ID:       99,
		Name:     "test_login",
		Status:   "FAILED",
		Type:     "STEP",
		LaunchID: 42,
		Issue: &struct {
			IssueType            string `json:"issueType"`
			Comment              string `json:"comment"`
			ExternalSystemIssues []struct {
				TicketID   string `json:"ticketId"`
				BtsURL     string `json:"btsUrl"`
				BtsProject string `json:"btsProject"`
				URL        string `json:"url"`
			} `json:"externalSystemIssues"`
		}{
			IssueType: "pb001",
			Comment:   "product bug",
		},
	}

	got := ti.toDomain("https://rp.example.com", "myproject")
	if got.ID != "99" {
		t.Errorf("ID = %q, want %q", got.ID, "99")
	}
	if got.IssueType != "pb001" {
		t.Errorf("IssueType = %q, want %q", got.IssueType, "pb001")
	}
	if got.Comment != "product bug" {
		t.Errorf("Comment = %q, want %q", got.Comment, "product bug")
	}
}

func TestRpTestItemToDomain_NoIssue(t *testing.T) {
	ti := &rpTestItem{
		ID:     100,
		Name:   "test_passing",
		Status: "PASSED",
		Type:   "STEP",
	}

	got := ti.toDomain("https://rp.example.com", "myproject")
	if got.IssueType != "" {
		t.Errorf("IssueType = %q, want empty for no issue", got.IssueType)
	}
}
