package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpdriver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
)

// mockService implements mcpdriver.EmceeService for testing.
type mockService struct {
	issues []domain.Issue
}

func (m *mockService) List(_ context.Context, backend string, filter domain.ListFilter) ([]domain.Issue, error) {
	if backend == "fail" {
		return nil, errors.New("backend not found")
	}
	return m.issues, nil
}

func (m *mockService) Get(_ context.Context, ref string) (*domain.Issue, error) {
	for _, i := range m.issues {
		if i.Ref == ref {
			return &i, nil
		}
	}
	return nil, errors.New("not found: " + ref)
}

func (m *mockService) Create(_ context.Context, backend string, input domain.CreateInput) (*domain.Issue, error) {
	issue := domain.Issue{Ref: backend + ":NEW-1", Key: "NEW-1", Title: input.Title}
	return &issue, nil
}

func (m *mockService) Update(_ context.Context, ref string, input domain.UpdateInput) (*domain.Issue, error) {
	for _, i := range m.issues {
		if i.Ref == ref {
			if input.Title != nil {
				i.Title = *input.Title
			}
			return &i, nil
		}
	}
	return nil, errors.New("not found: " + ref)
}

func (m *mockService) Search(_ context.Context, backend string, query string, limit int) ([]domain.Issue, error) {
	return m.issues, nil
}

func (m *mockService) ListChildren(_ context.Context, ref string) ([]domain.Issue, error) {
	return nil, nil
}

func (m *mockService) Backends() []string { return []string{"test"} }

func (m *mockService) ListDocuments(_ context.Context, backend string, filter domain.DocumentListFilter) ([]domain.Document, error) {
	return []domain.Document{{ID: "d1", Title: "Doc One"}}, nil
}

func (m *mockService) CreateDocument(_ context.Context, backend string, input domain.DocumentCreateInput) (*domain.Document, error) {
	return &domain.Document{ID: "d1", Title: input.Title, Content: input.Content}, nil
}

func (m *mockService) ListProjects(_ context.Context, backend string, filter domain.ProjectListFilter) ([]domain.Project, error) {
	return []domain.Project{{ID: "p1", Name: "Project One"}}, nil
}

func (m *mockService) CreateProject(_ context.Context, backend string, input domain.ProjectCreateInput) (*domain.Project, error) {
	return &domain.Project{ID: "p1", Name: input.Name}, nil
}

func (m *mockService) ListInitiatives(_ context.Context, backend string, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	return []domain.Initiative{{ID: "i1", Name: "Init One"}}, nil
}

func (m *mockService) CreateInitiative(_ context.Context, backend string, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	return &domain.Initiative{ID: "i1", Name: input.Name}, nil
}

func (m *mockService) ListLabels(_ context.Context, backend string) ([]domain.Label, error) {
	return []domain.Label{{ID: "l1", Name: "bug"}}, nil
}

func (m *mockService) CreateLabel(_ context.Context, backend string, input domain.LabelCreateInput) (*domain.Label, error) {
	return &domain.Label{ID: "l1", Name: input.Name}, nil
}

func (m *mockService) BulkCreateIssues(_ context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error) {
	var created []domain.Issue
	for i, input := range inputs {
		created = append(created, domain.Issue{Ref: fmt.Sprintf("%s:BULK-%d", backend, i+1), Title: input.Title})
	}
	return &domain.BulkCreateResult{Created: created, Total: len(inputs), Batches: 1}, nil
}

func (m *mockService) BulkUpdateIssues(_ context.Context, backend string, inputs []domain.BulkUpdateInput) (*domain.BulkUpdateResult, error) {
	var updated []domain.Issue
	for _, input := range inputs {
		issue := domain.Issue{Ref: input.Ref}
		if input.Title != nil {
			issue.Title = *input.Title
		}
		updated = append(updated, issue)
	}
	return &domain.BulkUpdateResult{Updated: updated, Total: len(inputs)}, nil
}

func newTestMCPServer() *server.MCPServer {
	svc := &mockService{
		issues: []domain.Issue{
			{Ref: "test:T-1", Key: "T-1", Title: "First Issue", Status: domain.StatusTodo, Priority: domain.PriorityHigh},
			{Ref: "test:T-2", Key: "T-2", Title: "Second Issue", Status: domain.StatusDone, Priority: domain.PriorityLow},
		},
	}
	s := server.NewMCPServer("emcee-test", "0.0.1")
	mcpdriver.RegisterToolsForTesting(s, svc)
	return s
}

func callTool(t *testing.T, s *server.MCPServer, name string, args map[string]any) *gomcp.CallToolResult {
	t.Helper()
	result := s.HandleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0", "id": 1,
		"method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	}))
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	var resp struct {
		Result gomcp.CallToolResult `json:"result"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("unmarshal response: %v\nraw: %s", err, data)
	}
	return &resp.Result
}

func initMCP(t *testing.T, s *server.MCPServer) {
	t.Helper()
	s.HandleMessage(context.Background(), mustJSON(t, map[string]any{
		"jsonrpc": "2.0", "id": 0, "method": "initialize",
		"params": map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{},
			"clientInfo":      map[string]any{"name": "test", "version": "1.0"},
		},
	}))
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	return data
}

// --- emcee tool tests ---

func TestEmceeList(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var issues []domain.Issue
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("got %d issues, want 2", len(issues))
	}
}

func TestEmceeGet(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "get", "ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "First Issue" {
		t.Errorf("title = %q, want %q", issue.Title, "First Issue")
	}
}

func TestEmceeGetMissingRef(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "get"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

func TestEmceeCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "create", "backend": "test", "title": "New thing"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "New thing" {
		t.Errorf("title = %q, want %q", issue.Title, "New thing")
	}
}

func TestEmceeUpdate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "update", "ref": "test:T-1", "title": "Updated"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestEmceeSearch(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "search", "backend": "test", "query": "first"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestEmceeBulkCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	issues := `[{"title":"Bulk 1"},{"title":"Bulk 2"}]`
	result := callTool(t, s, "emcee", map[string]any{"action": "bulk_create", "backend": "test", "issues": issues})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var bulkResult domain.BulkCreateResult
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &bulkResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bulkResult.Total != 2 {
		t.Errorf("total = %d, want 2", bulkResult.Total)
	}
}

func TestEmceeBulkUpdate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	issues := `[{"ref":"test:T-1","title":"Updated 1"},{"ref":"test:T-2","title":"Updated 2"}]`
	result := callTool(t, s, "emcee", map[string]any{"action": "bulk_update", "backend": "test", "issues": issues})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var bulkResult domain.BulkUpdateResult
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &bulkResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bulkResult.Total != 2 {
		t.Errorf("total = %d, want 2", bulkResult.Total)
	}
}

func TestEmceeUnknownAction(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee", map[string]any{"action": "invalid"})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
}

// --- emcee_manage tool tests ---

func TestManageDocList(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "doc_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var docs []domain.Document
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &docs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("got %d docs, want 1", len(docs))
	}
}

func TestManageDocCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "doc_create", "backend": "test", "title": "New Doc", "content": "body"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	var doc domain.Document
	if err := json.Unmarshal([]byte(result.Content[0].(gomcp.TextContent).Text), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Title != "New Doc" {
		t.Errorf("title = %q, want %q", doc.Title, "New Doc")
	}
}

func TestManageProjectList(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "project_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageProjectCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "project_create", "backend": "test", "name": "New Proj"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageInitiativeList(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "initiative_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageInitiativeCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "initiative_create", "backend": "test", "name": "New Init"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageLabelList(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "label_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageLabelCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "label_create", "backend": "test", "name": "urgent"})
	if result.IsError {
		t.Fatalf("error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
}

func TestManageUnknownAction(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)
	result := callTool(t, s, "emcee_manage", map[string]any{"action": "invalid"})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
}
