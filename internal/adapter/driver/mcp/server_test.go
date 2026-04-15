package mcp_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
	"github.com/DanyPops/emcee/internal/port/driver/drivertest"
	"github.com/dpopsuev/battery/mcpserver"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	mcpdriver "github.com/DanyPops/emcee/internal/adapter/driver/mcp"
)

func connectClient(t *testing.T, srv *mcpserver.Server) *sdkmcp.ClientSession {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)

	serverTransport, clientTransport := sdkmcp.NewInMemoryTransports()
	go func() { _ = srv.Serve(ctx, serverTransport) }()

	client := sdkmcp.NewClient(
		&sdkmcp.Implementation{Name: "test-client", Version: "v0.0.1"},
		nil,
	)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

func newTestServer(t *testing.T) (*sdkmcp.ClientSession, *drivertest.StubEmceeService) {
	t.Helper()
	svc := &drivertest.StubEmceeService{}
	svc.StubIssueService.Issues = []domain.Issue{
		{Ref: "test:T-1", Key: "T-1", Title: "First Issue", Status: domain.StatusTodo, Priority: domain.PriorityHigh},
		{Ref: "test:T-2", Key: "T-2", Title: "Second Issue", Status: domain.StatusDone, Priority: domain.PriorityLow},
	}
	svc.StubIssueService.Issue = &domain.Issue{Ref: "test:T-1", Key: "T-1", Title: "First Issue"}
	svc.StubIssueService.BackendList = []string{"test"}
	svc.StubDocumentService.Documents = []domain.Document{{ID: "d1", Title: "Doc One"}}
	svc.StubDocumentService.Document = &domain.Document{ID: "d1", Title: "New Doc"}
	svc.StubProjectService.Projects = []domain.Project{{ID: "p1", Name: "Project One"}}
	svc.StubProjectService.Project = &domain.Project{ID: "p1", Name: "New Proj"}
	svc.StubInitiativeService.Initiatives = []domain.Initiative{{ID: "i1", Name: "Init One"}}
	svc.StubInitiativeService.Initiative = &domain.Initiative{ID: "i1", Name: "New Init"}
	svc.StubLabelService.Labels = []domain.Label{{ID: "l1", Name: "bug"}}
	svc.StubLabelService.Label = &domain.Label{ID: "l1", Name: "urgent"}
	svc.StubBulkService.CreateResult = &domain.BulkCreateResult{
		Created: []domain.Issue{{Ref: "test:BULK-1", Title: "Bulk 1"}, {Ref: "test:BULK-2", Title: "Bulk 2"}},
		Total:   2,
		Batches: 1,
	}
	svc.StubBulkService.UpdateResult = &domain.BulkUpdateResult{
		Updated: []domain.Issue{{Ref: "test:T-1", Title: "Updated 1"}, {Ref: "test:T-2", Title: "Updated 2"}},
		Total:   2,
	}
	svc.StubHealthService.Status = &driver.HealthStatus{
		Status:   "healthy",
		Backends: []driver.BackendHealth{{Name: "test", Configured: true, Status: "healthy"}},
	}

	srv := mcpserver.NewServer("emcee-test", "0.0.1").
		WithInitTimeout(0)
	mcpdriver.RegisterTools(srv, svc)

	session := connectClient(t, srv)
	return session, svc
}

func callTool(t *testing.T, session *sdkmcp.ClientSession, name string, args map[string]any) *sdkmcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &sdkmcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return result
}

func resultText(t *testing.T, result *sdkmcp.CallToolResult) string {
	t.Helper()
	if len(result.Content) == 0 {
		t.Fatal("empty result content")
	}
	tc, ok := result.Content[0].(*sdkmcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", result.Content[0])
	}
	return tc.Text
}

// --- emcee tool tests ---

func TestEmceeList(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issues []domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 2 {
		t.Errorf("got %d issues, want 2", len(issues))
	}
}

func TestEmceeGet(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "get", "ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "First Issue" {
		t.Errorf("title = %q, want %q", issue.Title, "First Issue")
	}
}

func TestEmceeGetMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "get"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

func TestEmceeCreate(t *testing.T) {
	session, svc := newTestServer(t)
	svc.StubIssueService.Issue = &domain.Issue{Ref: "test:NEW-1", Key: "NEW-1", Title: "New thing"}
	result := callTool(t, session, "emcee", map[string]any{"action": "create", "backend": "test", "title": "New thing"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if issue.Title != "New thing" {
		t.Errorf("title = %q, want %q", issue.Title, "New thing")
	}
}

func TestEmceeUpdate(t *testing.T) {
	session, svc := newTestServer(t)
	svc.StubIssueService.Issue = &domain.Issue{Ref: "test:T-1", Key: "T-1", Title: "Updated"}
	result := callTool(t, session, "emcee", map[string]any{"action": "update", "ref": "test:T-1", "title": "Updated"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestEmceeSearch(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "search", "backend": "test", "query": "first"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestEmceeBulkCreate(t *testing.T) {
	session, _ := newTestServer(t)
	issues := `[{"title":"Bulk 1"},{"title":"Bulk 2"}]`
	result := callTool(t, session, "emcee", map[string]any{"action": "bulk_create", "backend": "test", "issues": issues})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var bulkResult domain.BulkCreateResult
	if err := json.Unmarshal([]byte(resultText(t, result)), &bulkResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bulkResult.Total != 2 {
		t.Errorf("total = %d, want 2", bulkResult.Total)
	}
}

func TestEmceeBulkUpdate(t *testing.T) {
	session, _ := newTestServer(t)
	issues := `[{"ref":"test:T-1","title":"Updated 1"},{"ref":"test:T-2","title":"Updated 2"}]`
	result := callTool(t, session, "emcee", map[string]any{"action": "bulk_update", "backend": "test", "issues": issues})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var bulkResult domain.BulkUpdateResult
	if err := json.Unmarshal([]byte(resultText(t, result)), &bulkResult); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if bulkResult.Total != 2 {
		t.Errorf("total = %d, want 2", bulkResult.Total)
	}
}

func TestEmceeUnknownAction(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "invalid"})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
}

// --- emcee_manage tool tests ---

func TestManageDocList(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "doc_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var docs []domain.Document
	if err := json.Unmarshal([]byte(resultText(t, result)), &docs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(docs) != 1 {
		t.Errorf("got %d docs, want 1", len(docs))
	}
}

func TestManageDocCreate(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "doc_create", "backend": "test", "title": "New Doc", "content": "body"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var doc domain.Document
	if err := json.Unmarshal([]byte(resultText(t, result)), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Title != "New Doc" {
		t.Errorf("title = %q, want %q", doc.Title, "New Doc")
	}
}

func TestManageProjectList(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "project_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageProjectCreate(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "project_create", "backend": "test", "name": "New Proj"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageInitiativeList(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "initiative_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageInitiativeCreate(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "initiative_create", "backend": "test", "name": "New Init"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageLabelList(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "label_list", "backend": "test"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageLabelCreate(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "label_create", "backend": "test", "name": "urgent"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestManageUnknownAction(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "invalid"})
	if !result.IsError {
		t.Fatal("expected error for unknown action")
	}
}

// --- emcee_health tool tests ---

func TestHealthTool(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_health", map[string]any{})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var health driver.HealthStatus
	if err := json.Unmarshal([]byte(resultText(t, result)), &health); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}
	if health.Status != "healthy" {
		t.Errorf("expected status=healthy, got %s", health.Status)
	}
	if len(health.Backends) != 1 {
		t.Errorf("expected 1 backend, got %d", len(health.Backends))
	}
}

func TestHealthToolDegraded(t *testing.T) {
	session, svc := newTestServer(t)
	svc.StubHealthService.Status = &driver.HealthStatus{
		Status:   "degraded",
		Backends: []driver.BackendHealth{},
		Warnings: []string{"No backends configured"},
	}
	result := callTool(t, session, "emcee_health", map[string]any{})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var health driver.HealthStatus
	if err := json.Unmarshal([]byte(resultText(t, result)), &health); err != nil {
		t.Fatalf("failed to parse health response: %v", err)
	}
	if health.Status != "degraded" {
		t.Errorf("expected status=degraded, got %s", health.Status)
	}
	if len(health.Warnings) == 0 {
		t.Error("expected warnings about no backends")
	}
}

// --- spy assertion test ---

func TestEmceeListSpyRecording(t *testing.T) {
	session, svc := newTestServer(t)
	_ = callTool(t, session, "emcee", map[string]any{"action": "list", "backend": "test", "status": "done", "limit": float64(10)})
	if len(svc.StubIssueService.ListCalls) != 1 {
		t.Fatalf("ListCalls = %d, want 1", len(svc.StubIssueService.ListCalls))
	}
	got := svc.StubIssueService.ListCalls[0]
	if got.Backend != "test" {
		t.Errorf("backend = %q, want %q", got.Backend, "test")
	}
	if got.Filter.Status != domain.StatusDone {
		t.Errorf("filter.Status = %q, want %q", got.Filter.Status, domain.StatusDone)
	}
	if got.Filter.Limit != 10 {
		t.Errorf("filter.Limit = %d, want 10", got.Filter.Limit)
	}
}

// --- tools/list test ---

func TestToolsList(t *testing.T) {
	session, _ := newTestServer(t)
	tools, err := session.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make(map[string]bool)
	for _, tool := range tools.Tools {
		names[tool.Name] = true
	}
	for _, want := range []string{"emcee", "emcee_manage", "emcee_health"} {
		if !names[want] {
			t.Errorf("missing tool %q in tools/list (got %v)", want, names)
		}
	}
	if len(tools.Tools) != 3 {
		t.Errorf("got %d tools, want 3", len(tools.Tools))
	}
	// Verify schemas have required field
	for _, tool := range tools.Tools {
		if tool.Name == "emcee" || tool.Name == "emcee_manage" {
			schema, _ := json.Marshal(tool.InputSchema)
			if len(schema) < 10 {
				t.Errorf("tool %s has empty schema", tool.Name)
			}
		}
	}
	_ = "" // satisfy import
}
