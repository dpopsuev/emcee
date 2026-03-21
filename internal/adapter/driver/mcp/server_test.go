package mcp_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/DanyPops/emcee/internal/domain"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	mcpdriver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
)

// mockService implements driver.IssueService for testing.
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
	issue := domain.Issue{
		Ref:   backend + ":NEW-1",
		Key:   "NEW-1",
		Title: input.Title,
	}
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

func (m *mockService) Backends() []string {
	return []string{"test"}
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
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "tools/call",
		"params":  map[string]any{"name": name, "arguments": args},
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

func TestMCPList(t *testing.T) {
	s := newTestMCPServer()
	// Must initialize first
	initMCP(t, s)

	result := callTool(t, s, "emcee_list", map[string]any{"backend": "test"})
	if result.IsError {
		t.Fatalf("expected success, got error: %s", result.Content[0].(gomcp.TextContent).Text)
	}
	text := result.Content[0].(gomcp.TextContent).Text
	var issues []domain.Issue
	if err := json.Unmarshal([]byte(text), &issues); err != nil {
		t.Fatalf("unmarshal issues: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("got %d issues, want 2", len(issues))
	}
}

func TestMCPGet(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)

	result := callTool(t, s, "emcee_get", map[string]any{"ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("expected success, got error")
	}
	text := result.Content[0].(gomcp.TextContent).Text
	var issue domain.Issue
	if err := json.Unmarshal([]byte(text), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "First Issue" {
		t.Errorf("title = %q, want %q", issue.Title, "First Issue")
	}
}

func TestMCPGetMissingRef(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)

	result := callTool(t, s, "emcee_get", map[string]any{})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

func TestMCPCreate(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)

	result := callTool(t, s, "emcee_create", map[string]any{"backend": "test", "title": "New thing"})
	if result.IsError {
		t.Fatalf("expected success, got error")
	}
	text := result.Content[0].(gomcp.TextContent).Text
	var issue domain.Issue
	if err := json.Unmarshal([]byte(text), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "New thing" {
		t.Errorf("title = %q, want %q", issue.Title, "New thing")
	}
}

func TestMCPSearch(t *testing.T) {
	s := newTestMCPServer()
	initMCP(t, s)

	result := callTool(t, s, "emcee_search", map[string]any{"backend": "test", "query": "first"})
	if result.IsError {
		t.Fatalf("expected success, got error")
	}
}
