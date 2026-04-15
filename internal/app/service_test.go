package app_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/DanyPops/emcee/internal/app"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/DanyPops/emcee/internal/port/driven/driventest"
)

// mockRepo implements all driven repository interfaces for testing.
type mockRepo struct {
	name        string
	issues      []domain.Issue
	docs        []domain.Document
	projects    []domain.Project
	initiatives []domain.Initiative
	labels      []domain.Label
}

var _ driven.IssueRepository = (*mockRepo)(nil)
var _ driven.DocumentRepository = (*mockRepo)(nil)
var _ driven.ProjectRepository = (*mockRepo)(nil)
var _ driven.InitiativeRepository = (*mockRepo)(nil)
var _ driven.LabelRepository = (*mockRepo)(nil)
var _ driven.BulkIssueRepository = (*mockRepo)(nil)

func (m *mockRepo) Name() string { return m.name }

func (m *mockRepo) List(_ context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	out := make([]domain.Issue, 0, len(m.issues))
	for i := range m.issues {
		if filter.Status != "" && m.issues[i].Status != filter.Status {
			continue
		}
		out = append(out, m.issues[i])
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) Get(_ context.Context, key string) (*domain.Issue, error) {
	for i := range m.issues {
		if m.issues[i].Key == key {
			return &m.issues[i], nil
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
	for i := range m.issues {
		if m.issues[i].Key == key {
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

func (m *mockRepo) ListChildren(_ context.Context, key string) ([]domain.Issue, error) {
	return nil, nil
}

func (m *mockRepo) ListDocuments(_ context.Context, filter domain.DocumentListFilter) ([]domain.Document, error) {
	out := make([]domain.Document, len(m.docs))
	copy(out, m.docs)
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) CreateDocument(_ context.Context, input domain.DocumentCreateInput) (*domain.Document, error) {
	doc := domain.Document{ID: "doc-1", Title: input.Title, Content: input.Content}
	m.docs = append(m.docs, doc)
	return &doc, nil
}

func (m *mockRepo) ListProjects(_ context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	out := make([]domain.Project, len(m.projects))
	copy(out, m.projects)
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) CreateProject(_ context.Context, input domain.ProjectCreateInput) (*domain.Project, error) {
	proj := domain.Project{ID: "proj-1", Name: input.Name, Description: input.Description}
	m.projects = append(m.projects, proj)
	return &proj, nil
}

func (m *mockRepo) UpdateProject(_ context.Context, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	for i := range m.projects {
		if m.projects[i].ID == id {
			if input.Name != nil {
				m.projects[i].Name = *input.Name
			}
			if input.Description != nil {
				m.projects[i].Description = *input.Description
			}
			return &m.projects[i], nil
		}
	}
	return nil, fmt.Errorf("project %s not found", id)
}

func (m *mockRepo) ListInitiatives(_ context.Context, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	out := make([]domain.Initiative, len(m.initiatives))
	copy(out, m.initiatives)
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, nil
}

func (m *mockRepo) CreateInitiative(_ context.Context, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	init := domain.Initiative{ID: "init-1", Name: input.Name, Description: input.Description}
	m.initiatives = append(m.initiatives, init)
	return &init, nil
}

func (m *mockRepo) ListLabels(_ context.Context) ([]domain.Label, error) {
	return m.labels, nil
}

func (m *mockRepo) CreateLabel(_ context.Context, input domain.LabelCreateInput) (*domain.Label, error) {
	label := domain.Label{ID: "label-1", Name: input.Name}
	m.labels = append(m.labels, label)
	return &label, nil
}

func (m *mockRepo) BulkCreateIssues(_ context.Context, inputs []domain.CreateInput) ([]domain.Issue, error) {
	created := make([]domain.Issue, 0, len(inputs))
	for i := range inputs {
		issue := domain.Issue{
			Ref:   fmt.Sprintf("%s:BULK-%d", m.name, i+1),
			ID:    fmt.Sprintf("bulk-id-%d", i+1),
			Key:   fmt.Sprintf("BULK-%d", i+1),
			Title: inputs[i].Title,
		}
		created = append(created, issue)
	}
	return created, nil
}

func newTestService() *app.Service {
	return app.NewService(&mockRepo{
		name: "test",
		issues: []domain.Issue{
			{Ref: "test:T-1", Key: "T-1", Title: "First", Status: domain.StatusTodo},
			{Ref: "test:T-2", Key: "T-2", Title: "Second", Status: domain.StatusDone},
		},
		docs: []domain.Document{
			{ID: "d1", Title: "Doc One"},
		},
		projects: []domain.Project{
			{ID: "p1", Name: "Project One"},
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

func TestServiceCreateDocument(t *testing.T) {
	svc := newTestService()
	doc, err := svc.CreateDocument(context.Background(), "test", domain.DocumentCreateInput{Title: "New doc", Content: "body"})
	if err != nil {
		t.Fatalf("CreateDocument: %v", err)
	}
	if doc.Title != "New doc" {
		t.Errorf("title = %q, want %q", doc.Title, "New doc")
	}
}

func TestServiceListDocuments(t *testing.T) {
	svc := newTestService()
	docs, err := svc.ListDocuments(context.Background(), "test", domain.DocumentListFilter{})
	if err != nil {
		t.Fatalf("ListDocuments: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("len = %d, want 1", len(docs))
	}
}

func TestServiceCreateProject(t *testing.T) {
	svc := newTestService()
	proj, err := svc.CreateProject(context.Background(), "test", domain.ProjectCreateInput{Name: "New proj"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if proj.Name != "New proj" {
		t.Errorf("name = %q, want %q", proj.Name, "New proj")
	}
}

func TestServiceListProjects(t *testing.T) {
	svc := newTestService()
	projs, err := svc.ListProjects(context.Background(), "test", domain.ProjectListFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projs) != 1 {
		t.Errorf("len = %d, want 1", len(projs))
	}
}

func TestServiceBulkCreateIssues(t *testing.T) {
	svc := newTestService()
	inputs := make([]domain.CreateInput, 120)
	for i := range inputs {
		inputs[i] = domain.CreateInput{Title: fmt.Sprintf("Issue %d", i+1)}
	}
	result, err := svc.BulkCreateIssues(context.Background(), "test", inputs)
	if err != nil {
		t.Fatalf("BulkCreateIssues: %v", err)
	}
	if result.Total != 120 {
		t.Errorf("total = %d, want 120", result.Total)
	}
	if result.Batches != 3 {
		t.Errorf("batches = %d, want 3", result.Batches)
	}
	if len(result.Created) != 120 {
		t.Errorf("created = %d, want 120", len(result.Created))
	}
}

func TestServiceDocumentsUnsupportedBackend(t *testing.T) {
	svc := newTestService()
	_, err := svc.ListDocuments(context.Background(), "unknown", domain.DocumentListFilter{})
	if err == nil {
		t.Fatal("expected error for unsupported backend")
	}
}

// --- Stub-based tests (spy assertion pattern) ---

func TestServiceListPassesFilter(t *testing.T) {
	stub := &driventest.StubCompositeRepository{}
	stub.StubIssueRepository.NameVal = "test"
	stub.StubIssueRepository.Issues = []domain.Issue{{Key: "T-1"}}
	svc := app.NewService(stub)

	filter := domain.ListFilter{Status: domain.StatusDone, Limit: 5}
	issues, err := svc.List(context.Background(), "test", filter)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("len = %d, want 1", len(issues))
	}
	if len(stub.StubIssueRepository.ListCalls) != 1 {
		t.Fatalf("ListCalls = %d, want 1", len(stub.StubIssueRepository.ListCalls))
	}
	got := stub.StubIssueRepository.ListCalls[0].Filter
	if got.Status != domain.StatusDone {
		t.Errorf("filter.Status = %q, want %q", got.Status, domain.StatusDone)
	}
	if got.Limit != 5 {
		t.Errorf("filter.Limit = %d, want 5", got.Limit)
	}
}

func TestServiceGetPassesKey(t *testing.T) {
	stub := &driventest.StubCompositeRepository{}
	stub.StubIssueRepository.NameVal = "test"
	stub.StubIssueRepository.Issue = &domain.Issue{Key: "T-1", Title: "Found"}
	svc := app.NewService(stub)

	issue, err := svc.Get(context.Background(), "test:T-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if issue.Title != "Found" {
		t.Errorf("title = %q, want %q", issue.Title, "Found")
	}
	if len(stub.StubIssueRepository.GetCalls) != 1 {
		t.Fatalf("GetCalls = %d, want 1", len(stub.StubIssueRepository.GetCalls))
	}
	if stub.StubIssueRepository.GetCalls[0].Key != "T-1" {
		t.Errorf("GetCalls[0].Key = %q, want %q", stub.StubIssueRepository.GetCalls[0].Key, "T-1")
	}
}

func TestServiceCreatePassesInput(t *testing.T) {
	stub := &driventest.StubCompositeRepository{}
	stub.StubIssueRepository.NameVal = "test"
	stub.StubIssueRepository.Issue = &domain.Issue{Key: "NEW-1", Title: "Created"}
	svc := app.NewService(stub)

	input := domain.CreateInput{Title: "Created", Priority: domain.PriorityHigh}
	issue, err := svc.Create(context.Background(), "test", input)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if issue.Title != "Created" {
		t.Errorf("title = %q, want %q", issue.Title, "Created")
	}
	if len(stub.StubIssueRepository.CreateCalls) != 1 {
		t.Fatalf("CreateCalls = %d, want 1", len(stub.StubIssueRepository.CreateCalls))
	}
	got := stub.StubIssueRepository.CreateCalls[0].Input
	if got.Title != "Created" {
		t.Errorf("input.Title = %q, want %q", got.Title, "Created")
	}
	if got.Priority != domain.PriorityHigh {
		t.Errorf("input.Priority = %v, want %v", got.Priority, domain.PriorityHigh)
	}
}
