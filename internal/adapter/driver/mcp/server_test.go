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
	svc.StubBuildService.BuildSummaries = []domain.BuildSummary{
		{Number: 99, URL: "https://jenkins.example.com/job/my-pipeline/99/"},
		{Number: 98, URL: "https://jenkins.example.com/job/my-pipeline/98/"},
	}
	svc.StubBuildService.LastBuild = &domain.Build{Number: 99, Result: domain.BuildSuccess}
	svc.StubBuildService.LastSuccessful = &domain.Build{Number: 97, Result: domain.BuildSuccess}
	svc.StubBuildService.LastFailed = &domain.Build{Number: 96, Result: domain.BuildFailure}
	svc.StubBuildService.JobParameters = []domain.JobParameter{
		{Name: "BRANCH", Type: "StringParameterDefinition", DefaultValue: "main", Description: "Branch to build"},
	}
	svc.StubBuildService.FolderJobs = []domain.Job{
		{Name: "sub-job-1", Buildable: true},
		{Name: "sub-job-2", Buildable: false},
	}
	svc.StubBuildService.UpstreamJobs = []domain.Job{{Name: "trigger-job"}}
	svc.StubBuildService.DownstreamJobs = []domain.Job{{Name: "deploy-job"}}
	svc.StubBuildService.Artifacts = []domain.BuildArtifact{
		{FileName: "app.jar", RelativePath: "target/app.jar"},
	}
	svc.StubBuildService.BuildRevision = "abc123def"
	svc.StubBuildService.BuildCauses = []domain.BuildCause{
		{ShortDescription: "Started by upstream project", UpstreamJob: "trigger-job", UpstreamBuild: 10},
	}
	svc.StubBuildService.Nodes = []domain.JenkinsNode{
		{Name: "master", Online: true, Idle: false, NumExecutors: 2, BusyExecutors: 1},
		{Name: "agent-1", Online: true, Idle: true, NumExecutors: 4, BusyExecutors: 0},
	}
	svc.StubBuildService.Node = &domain.JenkinsNode{Name: "agent-1", Online: true, Idle: true, NumExecutors: 4}
	svc.StubBuildService.Views = []domain.JenkinsView{
		{Name: "All", URL: "https://jenkins.example.com/view/All/"},
		{Name: "CI", URL: "https://jenkins.example.com/view/CI/", Jobs: []domain.Job{{Name: "ci-job"}}},
	}
	svc.StubBuildService.ViewJobs = []domain.Job{
		{Name: "ci-job", Buildable: true},
	}
	svc.StubPipelineService.PipelineRuns = []domain.PipelineRun{
		{ID: "1", Name: "#1", Status: "SUCCESS", Duration: 12345, Stages: []domain.PipelineStage{
			{ID: "10", Name: "Build", Status: "SUCCESS", Duration: 5000},
		}},
		{ID: "2", Name: "#2", Status: "IN_PROGRESS", Duration: 0},
	}
	svc.StubPipelineService.PipelineRun = &domain.PipelineRun{
		ID: "1", Name: "#1", Status: "SUCCESS", Duration: 12345,
		Stages: []domain.PipelineStage{{ID: "10", Name: "Build", Status: "SUCCESS", Duration: 5000}},
	}
	svc.StubPipelineService.PipelineInputs = []domain.PipelineInput{
		{ID: "input-1", Message: "Proceed to deploy?"},
	}
	svc.StubCIService.Pipelines = []domain.CIPipeline{
		{ID: 500, Status: "success", Ref: "main"},
		{ID: 501, Status: "failed", Ref: "feat"},
	}
	svc.StubCIService.Pipeline = &domain.CIPipeline{ID: 500, Status: "success", Ref: "main"}
	svc.StubCIService.PipelineJobs = []domain.CIJob{
		{ID: 600, PipelineID: 500, Name: "build", Stage: "build", Status: "success"},
		{ID: 601, PipelineID: 500, Name: "test", Stage: "test", Status: "success"},
	}
	svc.StubCIService.JobLogText = "Job succeeded\nAll tests passed"
	svc.StubActionsService.WorkflowRuns = []domain.WorkflowRun{
		{ID: 100, Name: "CI", Status: "completed", Conclusion: "success", Branch: "main"},
		{ID: 101, Name: "CI", Status: "completed", Conclusion: "failure", Branch: "feat"},
	}
	svc.StubActionsService.WorkflowRun = &domain.WorkflowRun{ID: 100, Name: "CI", Status: "completed", Conclusion: "success"}
	svc.StubActionsService.RunJobs = []domain.WorkflowJob{
		{ID: 200, RunID: 100, Name: "build", Status: "completed", Conclusion: "success"},
		{ID: 201, RunID: 100, Name: "test", Status: "completed", Conclusion: "success"},
	}
	svc.StubActionsService.RunLogs = "Build succeeded\nAll tests passed"
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

// --- build history tests ---

func TestEmceeBuilds(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "builds", "backend": "jenkins", "query": "my-pipeline", "limit": float64(10)})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var builds []domain.BuildSummary
	if err := json.Unmarshal([]byte(resultText(t, result)), &builds); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(builds) != 2 {
		t.Errorf("got %d builds, want 2", len(builds))
	}
	if len(svc.StubBuildService.ListBuildsCalls) != 1 {
		t.Fatalf("ListBuildsCalls = %d, want 1", len(svc.StubBuildService.ListBuildsCalls))
	}
	got := svc.StubBuildService.ListBuildsCalls[0]
	if got.JobName != "my-pipeline" {
		t.Errorf("job = %q, want %q", got.JobName, "my-pipeline")
	}
}

func TestEmceeBuildLast(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_last", "backend": "jenkins", "query": "my-pipeline"})
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

func TestEmceeBuildLastFailed(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_last_failed", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var build domain.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if build.Result != domain.BuildFailure {
		t.Errorf("result = %q, want %q", build.Result, domain.BuildFailure)
	}
}

func TestEmceeBuildLastSuccessful(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_last_successful", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var build domain.Build
	if err := json.Unmarshal([]byte(resultText(t, result)), &build); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if build.Result != domain.BuildSuccess {
		t.Errorf("result = %q, want %q", build.Result, domain.BuildSuccess)
	}
}

// --- folder navigation tests ---

func TestEmceeFolderJobs(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "folder_jobs", "backend": "jenkins", "query": "my-folder"})
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

func TestEmceeJobUpstream(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "job_upstream", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.Job
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "trigger-job" {
		t.Errorf("got %v, want [{Name:trigger-job}]", jobs)
	}
}

func TestEmceeJobDownstream(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "job_downstream", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.Job
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 1 || jobs[0].Name != "deploy-job" {
		t.Errorf("got %v, want [{Name:deploy-job}]", jobs)
	}
}

// --- artifacts & traceability tests ---

func TestEmceeBuildArtifacts(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_artifacts", "backend": "jenkins", "query": "my-pipeline", "ref": "99"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var artifacts []domain.BuildArtifact
	if err := json.Unmarshal([]byte(resultText(t, result)), &artifacts); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(artifacts) != 1 {
		t.Errorf("got %d artifacts, want 1", len(artifacts))
	}
	if artifacts[0].FileName != "app.jar" {
		t.Errorf("filename = %q, want %q", artifacts[0].FileName, "app.jar")
	}
	if len(svc.StubBuildService.ListArtifactsCalls) != 1 {
		t.Fatalf("ListArtifactsCalls = %d, want 1", len(svc.StubBuildService.ListArtifactsCalls))
	}
}

func TestEmceeBuildRevision(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_revision", "backend": "jenkins", "query": "my-pipeline", "ref": "99"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	text := resultText(t, result)
	if !strings.Contains(text, "abc123def") {
		t.Errorf("expected revision abc123def in response, got: %s", text)
	}
	if len(svc.StubBuildService.GetBuildRevisionCalls) != 1 {
		t.Fatalf("GetBuildRevisionCalls = %d, want 1", len(svc.StubBuildService.GetBuildRevisionCalls))
	}
}

func TestEmceeBuildCauses(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_causes", "backend": "jenkins", "query": "my-pipeline", "ref": "99"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var causes []domain.BuildCause
	if err := json.Unmarshal([]byte(resultText(t, result)), &causes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(causes) != 1 {
		t.Errorf("got %d causes, want 1", len(causes))
	}
	if causes[0].UpstreamJob != "trigger-job" {
		t.Errorf("upstream_job = %q, want %q", causes[0].UpstreamJob, "trigger-job")
	}
	if len(svc.StubBuildService.GetBuildCausesCalls) != 1 {
		t.Fatalf("GetBuildCausesCalls = %d, want 1", len(svc.StubBuildService.GetBuildCausesCalls))
	}
}

// --- nodes & views tests ---

func TestEmceeNodes(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "nodes", "backend": "jenkins"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var nodes []domain.JenkinsNode
	if err := json.Unmarshal([]byte(resultText(t, result)), &nodes); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(nodes) != 2 {
		t.Errorf("got %d nodes, want 2", len(nodes))
	}
	if len(svc.StubBuildService.ListNodesCalls) != 1 {
		t.Fatalf("ListNodesCalls = %d, want 1", len(svc.StubBuildService.ListNodesCalls))
	}
}

func TestEmceeNodeGet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "node_get", "backend": "jenkins", "query": "agent-1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var node domain.JenkinsNode
	if err := json.Unmarshal([]byte(resultText(t, result)), &node); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if node.Name != "agent-1" {
		t.Errorf("name = %q, want %q", node.Name, "agent-1")
	}
	if len(svc.StubBuildService.GetNodeCalls) != 1 {
		t.Fatalf("GetNodeCalls = %d, want 1", len(svc.StubBuildService.GetNodeCalls))
	}
}

func TestEmceeViews(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "views", "backend": "jenkins"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var views []domain.JenkinsView
	if err := json.Unmarshal([]byte(resultText(t, result)), &views); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(views) != 2 {
		t.Errorf("got %d views, want 2", len(views))
	}
	if len(svc.StubBuildService.ListViewsCalls) != 1 {
		t.Fatalf("ListViewsCalls = %d, want 1", len(svc.StubBuildService.ListViewsCalls))
	}
}

func TestEmceeViewJobs(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "view_jobs", "backend": "jenkins", "query": "CI"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.Job
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 1 {
		t.Errorf("got %d jobs, want 1", len(jobs))
	}
	if jobs[0].Name != "ci-job" {
		t.Errorf("name = %q, want %q", jobs[0].Name, "ci-job")
	}
	if len(svc.StubBuildService.GetViewJobsCalls) != 1 {
		t.Fatalf("GetViewJobsCalls = %d, want 1", len(svc.StubBuildService.GetViewJobsCalls))
	}
}

// --- build control tests ---

func TestEmceeBuildStop(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "build_stop", "backend": "jenkins", "query": "my-pipeline", "ref": "99"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubBuildService.StopBuildCalls) != 1 {
		t.Fatalf("StopBuildCalls = %d, want 1", len(svc.StubBuildService.StopBuildCalls))
	}
	got := svc.StubBuildService.StopBuildCalls[0]
	if got.JobName != "my-pipeline" || got.Number != 99 {
		t.Errorf("got %+v, want job=my-pipeline number=99", got)
	}
}

func TestEmceeJobParams(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "job_params", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var params []domain.JobParameter
	if err := json.Unmarshal([]byte(resultText(t, result)), &params); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(params) != 1 {
		t.Errorf("got %d params, want 1", len(params))
	}
	if params[0].Name != "BRANCH" {
		t.Errorf("name = %q, want %q", params[0].Name, "BRANCH")
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

// --- pipeline action tests ---

func TestEmceePipelineRuns(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "pipeline_runs", "backend": "jenkins", "query": "my-pipeline"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var runs []domain.PipelineRun
	if err := json.Unmarshal([]byte(resultText(t, result)), &runs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("got %d runs, want 2", len(runs))
	}
	if len(svc.StubPipelineService.ListPipelineRunsCalls) != 1 {
		t.Fatalf("ListPipelineRunsCalls = %d, want 1", len(svc.StubPipelineService.ListPipelineRunsCalls))
	}
	got := svc.StubPipelineService.ListPipelineRunsCalls[0]
	if got.JobName != "my-pipeline" {
		t.Errorf("job = %q, want %q", got.JobName, "my-pipeline")
	}
}

func TestEmceePipelineRunGet(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "pipeline_run_get", "backend": "jenkins", "query": "my-pipeline", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var run domain.PipelineRun
	if err := json.Unmarshal([]byte(resultText(t, result)), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if run.ID != "1" {
		t.Errorf("id = %q, want %q", run.ID, "1")
	}
	if len(run.Stages) != 1 {
		t.Errorf("got %d stages, want 1", len(run.Stages))
	}
	if len(svc.StubPipelineService.GetPipelineRunCalls) != 1 {
		t.Fatalf("GetPipelineRunCalls = %d, want 1", len(svc.StubPipelineService.GetPipelineRunCalls))
	}
}

func TestEmceePipelineInputs(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "pipeline_inputs", "backend": "jenkins", "query": "my-pipeline", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var inputs []domain.PipelineInput
	if err := json.Unmarshal([]byte(resultText(t, result)), &inputs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(inputs) != 1 {
		t.Errorf("got %d inputs, want 1", len(inputs))
	}
	if inputs[0].Message != "Proceed to deploy?" {
		t.Errorf("message = %q, want %q", inputs[0].Message, "Proceed to deploy?")
	}
	if len(svc.StubPipelineService.GetPendingInputsCalls) != 1 {
		t.Fatalf("GetPendingInputsCalls = %d, want 1", len(svc.StubPipelineService.GetPendingInputsCalls))
	}
}

func TestEmceePipelineInputApprove(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "pipeline_input_approve", "backend": "jenkins", "query": "my-pipeline", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubPipelineService.ApproveInputCalls) != 1 {
		t.Fatalf("ApproveInputCalls = %d, want 1", len(svc.StubPipelineService.ApproveInputCalls))
	}
	got := svc.StubPipelineService.ApproveInputCalls[0]
	if got.JobName != "my-pipeline" || got.RunID != "1" {
		t.Errorf("got %+v, want job=my-pipeline run_id=1", got)
	}
}

func TestEmceePipelineInputAbort(t *testing.T) {
	session, svc := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "pipeline_input_abort", "backend": "jenkins", "query": "my-pipeline", "ref": "1"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if len(svc.StubPipelineService.AbortInputCalls) != 1 {
		t.Fatalf("AbortInputCalls = %d, want 1", len(svc.StubPipelineService.AbortInputCalls))
	}
	got := svc.StubPipelineService.AbortInputCalls[0]
	if got.JobName != "my-pipeline" || got.RunID != "1" {
		t.Errorf("got %+v, want job=my-pipeline run_id=1", got)
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

// --- GitHub Actions tests ---

func TestEmceeRuns(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "runs", "backend": "github"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var runs []domain.WorkflowRun
	if err := json.Unmarshal([]byte(resultText(t, result)), &runs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("len = %d, want 2", len(runs))
	}
}

func TestEmceeRunGet(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "run_get", "backend": "github", "ref": "100"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var run domain.WorkflowRun
	if err := json.Unmarshal([]byte(resultText(t, result)), &run); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if run.ID != 100 {
		t.Errorf("id = %d, want 100", run.ID)
	}
}

func TestEmceeRunJobs(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "run_jobs", "backend": "github", "ref": "100"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.WorkflowJob
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("len = %d, want 2", len(jobs))
	}
}

func TestEmceeRunLogs(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "run_logs", "backend": "github", "ref": "100"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "Build succeeded") {
		t.Errorf("logs missing expected content")
	}
}

func TestEmceeRunRerun(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "run_rerun", "backend": "github", "ref": "100"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "true") {
		t.Errorf("expected rerun confirmation")
	}
}

// --- GitLab CI tests ---

func TestEmceeCIPipelines(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ci_pipelines", "backend": "gitlab"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var pipelines []domain.CIPipeline
	if err := json.Unmarshal([]byte(resultText(t, result)), &pipelines); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(pipelines) != 2 {
		t.Errorf("len = %d, want 2", len(pipelines))
	}
}

func TestEmceeCIPipeline(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ci_pipeline", "backend": "gitlab", "ref": "500"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var pipeline domain.CIPipeline
	if err := json.Unmarshal([]byte(resultText(t, result)), &pipeline); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if pipeline.ID != 500 {
		t.Errorf("id = %d, want 500", pipeline.ID)
	}
}

func TestEmceeCIJobs(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ci_jobs", "backend": "gitlab", "ref": "500"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	var jobs []domain.CIJob
	if err := json.Unmarshal([]byte(resultText(t, result)), &jobs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(jobs) != 2 {
		t.Errorf("len = %d, want 2", len(jobs))
	}
}

func TestEmceeCIJobLog(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ci_job_log", "backend": "gitlab", "ref": "600"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "Job succeeded") {
		t.Errorf("log missing expected content")
	}
}

func TestEmceeCIRetry(t *testing.T) {
	session, _ := newTestServer(t)
	result := callTool(t, session, "emcee", map[string]any{"action": "ci_retry", "backend": "gitlab", "ref": "500"})
	if result.IsError {
		t.Fatalf("error: %s", resultText(t, result))
	}
	if !strings.Contains(resultText(t, result), "true") {
		t.Errorf("expected retry confirmation")
	}
}
