package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dpopsuev/battery/mcpserver"
	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
	"github.com/dpopsuev/emcee/internal/port/driver/drivertest"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"

	mcpdriver "github.com/dpopsuev/emcee/internal/adapter/driver/mcp"
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
	svc.StubPRService.PRs = []domain.PullRequest{
		{Number: 42, Title: "feat: add widget", Author: "alice", State: "merged", URL: "https://github.com/org/repo/pull/42"},
	}
	svc.StubFieldService.Fields = []domain.Field{
		{ID: "summary", Name: "Summary", Custom: false},
		{ID: "customfield_10001", Name: "Sprint", Custom: true},
	}
	svc.StubJQLService.Issues = []domain.Issue{
		{Ref: "jira:PROJ-1", Key: "PROJ-1", Title: "JQL result"},
	}
	svc.StubLaunchService.Launches = []domain.Launch{
		{ID: "1", Name: "Regression Suite", Status: "PASSED"},
		{ID: "2", Name: "Smoke Tests", Status: "FAILED"},
	}
	svc.StubLaunchService.Launch = &domain.Launch{ID: "1", Name: "Regression Suite", Status: "PASSED"}
	svc.StubLaunchService.TestItems = []domain.TestItem{
		{ID: "10", Name: "test_login", Status: "PASSED", LaunchID: "1"},
		{ID: "11", Name: "test_logout", Status: "FAILED", LaunchID: "1"},
	}
	svc.StubLaunchService.TestItem = &domain.TestItem{
		ID: "10", Name: "test_login", Status: "FAILED", LaunchID: "1",
		FailureMessage: "AssertionError: expected 200 got 401",
	}
	svc.StubLaunchService.BulkTestItems = []domain.TestItem{
		{ID: "10", Name: "test_login", Status: "FAILED", LaunchID: "1"},
		{ID: "11", Name: "test_logout", Status: "PASSED", LaunchID: "1"},
	}
	svc.StubIssueService.Issue = &domain.Issue{
		Ref: "test:T-1", Key: "T-1", Title: "First Issue",
		IssueLinks: []domain.IssueLink{
			{Type: "blocks", Direction: "outward", TargetRef: "test:T-2", TargetKey: "T-2", TargetTitle: "Second Issue"},
		},
		ExternalLinks: []domain.ExternalLink{
			{Title: "PR #42", URL: "https://github.com/org/repo/pull/42", Type: "GitHub"},
		},
	}
	svc.StubTriageService.Config = driver.TriageConfig{
		RateLimit: 5,
		AllowList: []string{"jira", "jenkins-ci"},
	}
	svc.StubTriageService.Graph = &domain.TriageGraph{
		Seed: "test:T-1",
		Nodes: []domain.TriageNode{
			{Ref: "test:T-1", Type: "issue", Phase: "stored", Title: "First Issue", Status: "todo"},
		},
		Edges: []domain.TriageEdge{
			{From: "test:T-1", To: "github:org/repo#42", Type: "mentions", Confidence: 0.9, Source: "description"},
		},
	}
	svc.StubLedgerService.Record = &domain.ArtifactRecord{
		Ref: "jira:BUG-1", Backend: "jira", Type: "issue", Title: "Bug One", Status: "open",
	}
	svc.StubLedgerService.Records = []domain.ArtifactRecord{
		{Ref: "jira:BUG-1", Backend: "jira", Type: "issue", Title: "Bug One", Status: "open"},
		{Ref: "github:org/repo#1", Backend: "github", Type: "issue", Title: "Fix stuff", Status: "closed"},
	}
	svc.StubLedgerService.SearchRecords = []domain.ArtifactRecord{
		{Ref: "jira:BUG-1", Backend: "jira", Type: "issue", Title: "Bug One", Status: "open"},
	}
	svc.StubLedgerService.SimilarRecords = []domain.ArtifactRecord{
		{Ref: "github:org/repo#1", Backend: "github", Type: "issue", Title: "Fix stuff", Status: "closed"},
	}
	svc.StubLedgerService.StatsResult = &domain.LedgerStats{
		Total:     2,
		ByBackend: map[string]int{"jira": 1, "github": 1},
	}
	svc.StubBackendManager.RemoveResult = true
	svc.StubBackendManager.ReloadAdded = []string{"jenkins-ci"}
	svc.StubBackendManager.ReloadRemoved = []string{"old-backend"}

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

// --- PR action tests ---

func TestEmceePRs(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "prs",
		"backend": "github",
		"author":  "alice",
		"status":  "merged",
		"repo":    "org/repo",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var prs []domain.PullRequest
	if err := json.Unmarshal([]byte(resultText(t, result)), &prs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(prs) != 1 {
		t.Errorf("got %d PRs, want 1", len(prs))
	}
	if len(svc.StubPRService.ListPRCalls) != 1 {
		t.Fatalf("ListPRCalls = %d, want 1", len(svc.StubPRService.ListPRCalls))
	}
	got := svc.StubPRService.ListPRCalls[0]
	if got.Filter.Author != "alice" {
		t.Errorf("author = %q, want %q", got.Filter.Author, "alice")
	}
	if got.Filter.Repo != "org/repo" {
		t.Errorf("repo = %q, want %q", got.Filter.Repo, "org/repo")
	}
}

// --- field discovery + JQL action tests ---

func TestEmceeFields(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "fields", "backend": "jira"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var fields []domain.Field
	if err := json.Unmarshal([]byte(resultText(t, result)), &fields); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(fields) != 2 {
		t.Errorf("got %d fields, want 2", len(fields))
	}
	if len(svc.StubFieldService.ListFieldsCalls) != 1 {
		t.Fatalf("ListFieldsCalls = %d, want 1", len(svc.StubFieldService.ListFieldsCalls))
	}
}

func TestEmceeJQL(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "jql",
		"backend": "jira",
		"query":   "project = PROJ AND status = Open",
		"limit":   float64(25),
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issues []domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issues); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issues) != 1 {
		t.Errorf("got %d issues, want 1", len(issues))
	}
	if len(svc.StubJQLService.SearchJQLCalls) != 1 {
		t.Fatalf("SearchJQLCalls = %d, want 1", len(svc.StubJQLService.SearchJQLCalls))
	}
	got := svc.StubJQLService.SearchJQLCalls[0]
	if got.JQL != "project = PROJ AND status = Open" {
		t.Errorf("jql = %q, want %q", got.JQL, "project = PROJ AND status = Open")
	}
	if got.Limit != 25 {
		t.Errorf("limit = %d, want 25", got.Limit)
	}
}

func TestEmceeJQLMissingQuery(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "jql", "backend": "jira"})
	if !result.IsError {
		t.Fatal("expected error for missing query")
	}
}

// --- triage tests ---

func TestEmceeTriage(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "triage", "ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var graph domain.TriageGraph
	if err := json.Unmarshal([]byte(resultText(t, result)), &graph); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if graph.Seed != "test:T-1" {
		t.Errorf("seed = %q, want %q", graph.Seed, "test:T-1")
	}
	if len(graph.Nodes) != 1 {
		t.Errorf("got %d nodes, want 1", len(graph.Nodes))
	}
	if len(graph.Edges) != 1 {
		t.Errorf("got %d edges, want 1", len(graph.Edges))
	}
	if len(svc.StubTriageService.TriageCalls) != 1 {
		t.Fatalf("TriageCalls = %d, want 1", len(svc.StubTriageService.TriageCalls))
	}
	got := svc.StubTriageService.TriageCalls[0]
	if got.Ref != "test:T-1" {
		t.Errorf("ref = %q, want %q", got.Ref, "test:T-1")
	}
	if got.MaxDepth != 3 {
		t.Errorf("maxDepth = %d, want 3 (default)", got.MaxDepth)
	}
}

func TestEmceeTriageMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "triage"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

// --- triage config tests ---

func TestEmceeTriageConfig(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "triage_config"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var cfg driver.TriageConfig
	if err := json.Unmarshal([]byte(resultText(t, result)), &cfg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if cfg.RateLimit != 5 {
		t.Errorf("rate_limit = %f, want 5", cfg.RateLimit)
	}
	if len(cfg.AllowList) != 2 {
		t.Errorf("allow_list = %v, want [jira jenkins-ci]", cfg.AllowList)
	}
}

func TestEmceeTriageConfigSet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action": "triage_config_set",
		"limit":  float64(20),
		"issues": `["jira","gitlab"]`,
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if svc.StubTriageService.Config.RateLimit != 20 {
		t.Errorf("rate_limit = %f, want 20", svc.StubTriageService.Config.RateLimit)
	}
	if len(svc.StubTriageService.Config.AllowList) != 2 {
		t.Errorf("allow_list = %v, want [jira gitlab]", svc.StubTriageService.Config.AllowList)
	}
}

// --- backend management tests ---

func TestManageConfigReload(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "config_reload"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubBackendManager.ReloadConfigCalls) != 1 {
		t.Fatalf("ReloadConfigCalls = %d, want 1", len(svc.StubBackendManager.ReloadConfigCalls))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "jenkins-ci") {
		t.Errorf("expected added backend in response, got: %s", text)
	}
}

func TestManageBackendRemove(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "backend_remove", "name": "old-jira"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubBackendManager.RemoveBackendCalls) != 1 {
		t.Fatalf("RemoveBackendCalls = %d, want 1", len(svc.StubBackendManager.RemoveBackendCalls))
	}
	if svc.StubBackendManager.RemoveBackendCalls[0].Name != "old-jira" {
		t.Errorf("name = %q, want %q", svc.StubBackendManager.RemoveBackendCalls[0].Name, "old-jira")
	}
}

func TestManageBackendRemoveMissingName(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee_manage", map[string]any{"action": "backend_remove"})
	if !result.IsError {
		t.Fatal("expected error for missing name")
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

// --- ledger action tests ---

func TestEmceeLedgerList(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_list", "backend": "jira"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var records []domain.ArtifactRecord
	if err := json.Unmarshal([]byte(resultText(t, result)), &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 2 {
		t.Errorf("got %d records, want 2", len(records))
	}
	if len(svc.StubLedgerService.LedgerListCalls) != 1 {
		t.Fatalf("LedgerListCalls = %d, want 1", len(svc.StubLedgerService.LedgerListCalls))
	}
	got := svc.StubLedgerService.LedgerListCalls[0]
	if got.Filter.Backend != "jira" {
		t.Errorf("filter.Backend = %q, want %q", got.Filter.Backend, "jira")
	}
}

func TestEmceeLedgerGet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_get", "ref": "jira:BUG-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var record domain.ArtifactRecord
	if err := json.Unmarshal([]byte(resultText(t, result)), &record); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if record.Title != "Bug One" {
		t.Errorf("title = %q, want %q", record.Title, "Bug One")
	}
	if len(svc.StubLedgerService.LedgerGetCalls) != 1 {
		t.Fatalf("LedgerGetCalls = %d, want 1", len(svc.StubLedgerService.LedgerGetCalls))
	}
	if svc.StubLedgerService.LedgerGetCalls[0].Ref != "jira:BUG-1" {
		t.Errorf("ref = %q, want %q", svc.StubLedgerService.LedgerGetCalls[0].Ref, "jira:BUG-1")
	}
}

func TestEmceeLedgerGetMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_get"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

func TestEmceeLedgerStats(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_stats"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var stats domain.LedgerStats
	if err := json.Unmarshal([]byte(resultText(t, result)), &stats); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if stats.Total != 2 {
		t.Errorf("total = %d, want 2", stats.Total)
	}
	if stats.ByBackend["jira"] != 1 {
		t.Errorf("by_backend[jira] = %d, want 1", stats.ByBackend["jira"])
	}
	if svc.StubLedgerService.LedgerStatsCalls != 1 {
		t.Errorf("LedgerStatsCalls = %d, want 1", svc.StubLedgerService.LedgerStatsCalls)
	}
}

func TestEmceeLedgerSearch(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action": "ledger_search",
		"query":  "Bug",
		"limit":  float64(10),
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var records []domain.ArtifactRecord
	if err := json.Unmarshal([]byte(resultText(t, result)), &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
	if records[0].Ref != "jira:BUG-1" {
		t.Errorf("ref = %q, want jira:BUG-1", records[0].Ref)
	}
	if len(svc.StubLedgerService.LedgerSearchCalls) != 1 {
		t.Fatalf("LedgerSearchCalls = %d, want 1", len(svc.StubLedgerService.LedgerSearchCalls))
	}
	if svc.StubLedgerService.LedgerSearchCalls[0].Query != "Bug" {
		t.Errorf("query = %q, want %q", svc.StubLedgerService.LedgerSearchCalls[0].Query, "Bug")
	}
}

func TestEmceeLedgerSearchMissingQuery(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_search"})
	if !result.IsError {
		t.Fatal("expected error for missing query")
	}
}

func TestEmceeLedgerIngest(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":      "ledger_ingest",
		"ref":         "jira:NEW-1",
		"backend":     "jira",
		"title":       "Manually ingested issue",
		"description": "Some description text",
		"status":      "open",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "jira:NEW-1") {
		t.Errorf("result does not contain ref: %s", text)
	}
	if len(svc.StubLedgerService.LedgerIngestCalls) != 1 {
		t.Fatalf("LedgerIngestCalls = %d, want 1", len(svc.StubLedgerService.LedgerIngestCalls))
	}
	rec := svc.StubLedgerService.LedgerIngestCalls[0].Record
	if rec.Ref != "jira:NEW-1" {
		t.Errorf("ref = %q, want jira:NEW-1", rec.Ref)
	}
	if rec.Title != "Manually ingested issue" {
		t.Errorf("title = %q, want %q", rec.Title, "Manually ingested issue")
	}
}

func TestEmceeLedgerIngestMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_ingest", "title": "No ref"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

func TestEmceeLedgerSimilar(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action": "ledger_similar",
		"ref":    "jira:BUG-1",
		"limit":  float64(5),
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var records []domain.ArtifactRecord
	if err := json.Unmarshal([]byte(resultText(t, result)), &records); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("len = %d, want 1", len(records))
	}
	if len(svc.StubLedgerService.LedgerSimilarCalls) != 1 {
		t.Fatalf("LedgerSimilarCalls = %d, want 1", len(svc.StubLedgerService.LedgerSimilarCalls))
	}
	if svc.StubLedgerService.LedgerSimilarCalls[0].Ref != "jira:BUG-1" {
		t.Errorf("ref = %q, want jira:BUG-1", svc.StubLedgerService.LedgerSimilarCalls[0].Ref)
	}
}

func TestEmceeLedgerSimilarMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ledger_similar"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
	}
}

// --- Issue links tests ---

func TestEmceeGetIncludesIssueLinks(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "get", "ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issue.IssueLinks) != 1 {
		t.Fatalf("issue_links len = %d, want 1", len(issue.IssueLinks))
	}
	if issue.IssueLinks[0].TargetKey != "T-2" {
		t.Errorf("target_key = %q, want T-2", issue.IssueLinks[0].TargetKey)
	}
}

func TestEmceeGetIncludesExternalLinks(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "get", "ref": "test:T-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var issue domain.Issue
	if err := json.Unmarshal([]byte(resultText(t, result)), &issue); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(issue.ExternalLinks) != 1 {
		t.Fatalf("external_links len = %d, want 1", len(issue.ExternalLinks))
	}
	if issue.ExternalLinks[0].Type != "GitHub" {
		t.Errorf("type = %q, want GitHub", issue.ExternalLinks[0].Type)
	}
}

func TestEmceeLinkIssue(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":     "link_issue",
		"ref":        "test:T-1",
		"query":      "T-2",
		"issue_type": "Blocks",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubIssueLinkService.LinkIssueCalls) != 1 {
		t.Fatalf("LinkIssueCalls = %d, want 1", len(svc.StubIssueLinkService.LinkIssueCalls))
	}
	call := svc.StubIssueLinkService.LinkIssueCalls[0]
	if call.InwardKey != "T-1" || call.OutwardKey != "T-2" || call.Type != "Blocks" {
		t.Errorf("unexpected call: %+v", call)
	}
}

// --- Report Portal tests ---

func TestEmceeLaunches(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "launches", "backend": "reportportal"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var launches []domain.Launch
	if err := json.Unmarshal([]byte(resultText(t, result)), &launches); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(launches) != 2 {
		t.Errorf("len = %d, want 2", len(launches))
	}
}

func TestEmceeLaunchGet(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "launch_get", "backend": "reportportal", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var launch domain.Launch
	if err := json.Unmarshal([]byte(resultText(t, result)), &launch); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if launch.Name != "Regression Suite" {
		t.Errorf("name = %q, want %q", launch.Name, "Regression Suite")
	}
}

func TestEmceeTestItems(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "test_items", "backend": "reportportal", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var items []domain.TestItem
	if err := json.Unmarshal([]byte(resultText(t, result)), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("len = %d, want 2", len(items))
	}
}

func TestEmceeTestItemGet(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "test_item_get", "backend": "reportportal", "ref": "10"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var item domain.TestItem
	if err := json.Unmarshal([]byte(resultText(t, result)), &item); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if item.FailureMessage != "AssertionError: expected 200 got 401" {
		t.Errorf("failure_message = %q", item.FailureMessage)
	}
}

func TestEmceeBulkTestItemGet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "bulk_test_item_get", "backend": "reportportal", "issues": `["10","11"]`})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var items []domain.TestItem
	if err := json.Unmarshal([]byte(resultText(t, result)), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len = %d, want 2", len(items))
	}
	if len(svc.StubLaunchService.GetTestItemsCalls) != 1 {
		t.Fatalf("GetTestItemsCalls = %d, want 1", len(svc.StubLaunchService.GetTestItemsCalls))
	}
}

func TestEmceeDefectUpdate(t *testing.T) {
	session, svc := newTestServer(t)
	updates := `[{"test_item_id":"10","issue_type":"pb001","comment":"product bug"}]`
	result := callTool(t, session, "emcee", map[string]any{"action": "defect_update", "backend": "reportportal", "issues": updates})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubLaunchService.UpdateDefectsCalls) != 1 {
		t.Fatalf("UpdateDefectsCalls = %d, want 1", len(svc.StubLaunchService.UpdateDefectsCalls))
	}
}
