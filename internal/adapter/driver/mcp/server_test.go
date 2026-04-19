package mcp_test

import (
	"context"
	"encoding/json"
	"strings"
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
	svc.StubLaunchService.Launches = []domain.Launch{
		{ID: "1", Name: "Regression Suite", Status: "PASSED"},
		{ID: "2", Name: "Smoke Tests", Status: "FAILED"},
	}
	svc.StubLaunchService.Launch = &domain.Launch{ID: "1", Name: "Regression Suite", Status: "PASSED"}
	svc.StubLaunchService.TestItems = []domain.TestItem{
		{ID: "10", Name: "test_login", Status: "PASSED", LaunchID: "1"},
		{ID: "11", Name: "test_logout", Status: "FAILED", LaunchID: "1"},
	}
	svc.StubLaunchService.TestItem = &domain.TestItem{ID: "10", Name: "test_login", Status: "PASSED", LaunchID: "1"}
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
	svc.StubBuildService.Jobs = []domain.Job{
		{Name: "my-pipeline", URL: "https://jenkins.example.com/job/my-pipeline", Buildable: true},
		{Name: "deploy-prod", URL: "https://jenkins.example.com/job/deploy-prod", Buildable: false},
	}
	svc.StubBuildService.Job = &domain.Job{Name: "my-pipeline", Buildable: true}
	svc.StubBuildService.BuildNumber = 42
	svc.StubBuildService.Build = &domain.Build{Number: 99, Result: domain.BuildSuccess}
	svc.StubBuildService.BuildLog = "BUILD SUCCESS\nFinished: SUCCESS"
	svc.StubBuildService.TestResult = &domain.TestResult{Passed: 10, Failed: 1, Skipped: 2, Total: 13}
	svc.StubBuildService.QueueItems = []domain.QueueItem{
		{ID: 1, TaskName: "my-pipeline", Blocked: false, Buildable: true},
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

// --- launch/RP action tests ---

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
		t.Errorf("got %d launches, want 2", len(launches))
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

func TestEmceeLaunchGetMissingRef(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "launch_get", "backend": "reportportal"})
	if !result.IsError {
		t.Fatal("expected error for missing ref")
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
		t.Errorf("got %d items, want 2", len(items))
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
	if item.Name != "test_login" {
		t.Errorf("name = %q, want %q", item.Name, "test_login")
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
	got := svc.StubLaunchService.UpdateDefectsCalls[0]
	if got.Backend != "reportportal" {
		t.Errorf("backend = %q, want %q", got.Backend, "reportportal")
	}
	if len(got.Updates) != 1 || got.Updates[0].TestItemID != "10" {
		t.Errorf("unexpected updates: %+v", got.Updates)
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

// --- build/Jenkins action tests ---

func TestEmceeJobs(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "jobs", "backend": "jenkins"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.Job
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("got %d jobs, want 2", len(jobs))
	}
}

func TestEmceeJobGet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "job_get", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var job domain.Job
	if err := json.Unmarshal([]byte(resultText(t, result)), &job); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if job.Name != "my-pipeline" {
		t.Errorf("name = %q, want %q", job.Name, "my-pipeline")
	}
	if len(svc.StubBuildService.GetJobCalls) != 1 {
		t.Fatalf("GetJobCalls = %d, want 1", len(svc.StubBuildService.GetJobCalls))
	}
}

func TestEmceeBuildTrigger(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "build_trigger",
		"backend": "jenkins",
		"query":   "my-pipeline",
		"issues":  `{"BRANCH":"main"}`,
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubBuildService.TriggerBuildCalls) != 1 {
		t.Fatalf("TriggerBuildCalls = %d, want 1", len(svc.StubBuildService.TriggerBuildCalls))
	}
	got := svc.StubBuildService.TriggerBuildCalls[0]
	if got.JobName != "my-pipeline" {
		t.Errorf("job = %q, want %q", got.JobName, "my-pipeline")
	}
	if got.Params["BRANCH"] != "main" {
		t.Errorf("params[BRANCH] = %q, want %q", got.Params["BRANCH"], "main")
	}
}

func TestEmceeBuildGet(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "build_get",
		"backend": "jenkins",
		"query":   "my-pipeline",
		"ref":     "99",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var build domain.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if build.Number != 99 {
		t.Errorf("number = %d, want 99", build.Number)
	}
}

func TestEmceeBuildLog(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "build_log",
		"backend": "jenkins",
		"query":   "my-pipeline",
		"ref":     "99",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
}

func TestEmceeTestResults(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{
		"action":  "test_results",
		"backend": "jenkins",
		"query":   "my-pipeline",
		"ref":     "99",
	})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var tr domain.TestResult
	if err := json.Unmarshal([]byte(resultText(t, result)), &tr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if tr.Total != 13 {
		t.Errorf("total = %d, want 13", tr.Total)
	}
}

func TestEmceeQueue(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "queue", "backend": "jenkins"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var items []domain.QueueItem
	if err := json.Unmarshal([]byte(resultText(t, result)), &items); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("got %d items, want 1", len(items))
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
