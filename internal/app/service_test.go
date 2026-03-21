package app_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/DanyPops/emcee/internal/app"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

// mockRepo implements driven.IssueRepository for testing.
type mockRepo struct {
	name   string
	issues []domain.Issue
}

var _ driven.IssueRepository = (*mockRepo)(nil)

func (m *mockRepo) Name() string { return m.name }

func (m *mockRepo) List(_ context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	var out []domain.Issue
	for _, i := range m.issues {
		if filter.Status != "" && i.Status != filter.Status {
			continue
		}
		out = append(out, i)
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) Get(_ context.Context, key string) (*domain.Issue, error) {
	for _, i := range m.issues {
		if i.Key == key {
			return &i, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockRepo) Create(_ context.Context, input domain.CreateInput) (*domain.Issue, error) {
	issue := domain.Issue{
		Ref:   m.name + ":NEW-1",
		ID:    "new-id",
		Key:   "NEW-1",
		Title: input.Title,
	}
	m.issues = append(m.issues, issue)
	return &issue, nil
}

func (m *mockRepo) Update(_ context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	for i, issue := range m.issues {
		if issue.Key == key {
			if input.Title != nil {
				m.issues[i].Title = *input.Title
			}
			if input.Status != nil {
				m.issues[i].Status = *input.Status
			}
			return &m.issues[i], nil
		}
	}
	return nil, fmt.Errorf("not found: %s", key)
}

func (m *mockRepo) Search(_ context.Context, query string, limit int) ([]domain.Issue, error) {
	return m.issues, nil
}

func newTestService() *app.Service {
	return app.NewService(&mockRepo{
		name: "test",
		issues: []domain.Issue{
			{Ref: "test:T-1", Key: "T-1", Title: "First", Status: domain.StatusTodo},
			{Ref: "test:T-2", Key: "T-2", Title: "Second", Status: domain.StatusDone},
		},
	})
}

func TestParseRef(t *testing.T) {
	tests := []struct {
		ref     string
		backend string
		key     string
		wantErr bool
	}{
		{"linear:HEG-17", "linear", "HEG-17", false},
		{"github:owner/repo#42", "github", "owner/repo#42", false},
		{"jira:PROJ-123", "jira", "PROJ-123", false},
		{"nocolon", "", "", true},
		{":nobackend", "", "", true},
		{"nokey:", "", "", true},
		{"", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			backend, key, err := app.ParseRef(tt.ref)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseRef(%q) expected error", tt.ref)
				}
				return
			}
			if err != nil {
				t.Errorf("ParseRef(%q) unexpected error: %v", tt.ref, err)
				return
			}
			if backend != tt.backend || key != tt.key {
				t.Errorf("ParseRef(%q) = (%q, %q), want (%q, %q)", tt.ref, backend, key, tt.backend, tt.key)
			}
		})
	}
}

func TestServiceGet(t *testing.T) {
	svc := newTestService()

	issue, err := svc.Get(context.Background(), "test:T-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Title != "First" {
		t.Errorf("title = %q, want %q", issue.Title, "First")
	}
}

func TestServiceGetUnknownBackend(t *testing.T) {
	svc := newTestService()

	_, err := svc.Get(context.Background(), "unknown:X-1")
	if err == nil {
		t.Fatal("expected error for unknown backend")
	}
}

func TestServiceGetNotFound(t *testing.T) {
	svc := newTestService()

	_, err := svc.Get(context.Background(), "test:NOPE-999")
	if err == nil {
		t.Fatal("expected error for missing issue")
	}
}

func TestServiceList(t *testing.T) {
	svc := newTestService()

	issues, err := svc.List(context.Background(), "test", domain.ListFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("len = %d, want 2", len(issues))
	}
}

func TestServiceListWithStatusFilter(t *testing.T) {
	svc := newTestService()

	issues, err := svc.List(context.Background(), "test", domain.ListFilter{Status: domain.StatusDone})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("len = %d, want 1", len(issues))
	}
	if issues[0].Key != "T-2" {
		t.Errorf("key = %q, want T-2", issues[0].Key)
	}
}

func TestServiceCreate(t *testing.T) {
	svc := newTestService()

	issue, err := svc.Create(context.Background(), "test", domain.CreateInput{Title: "New issue"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if issue.Title != "New issue" {
		t.Errorf("title = %q, want %q", issue.Title, "New issue")
	}
	if issue.Ref != "test:NEW-1" {
		t.Errorf("ref = %q, want %q", issue.Ref, "test:NEW-1")
	}
}

func TestServiceUpdate(t *testing.T) {
	svc := newTestService()
	newTitle := "Updated"

	issue, err := svc.Update(context.Background(), "test:T-1", domain.UpdateInput{Title: &newTitle})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if issue.Title != "Updated" {
		t.Errorf("title = %q, want %q", issue.Title, "Updated")
	}
}

func TestServiceBackends(t *testing.T) {
	svc := newTestService()
	backends := svc.Backends()
	if len(backends) != 1 || backends[0] != "test" {
		t.Errorf("backends = %v, want [test]", backends)
	}
}
