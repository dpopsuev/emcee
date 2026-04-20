// Package mcp implements the driver (inbound) adapter as an MCP stdio server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
	"github.com/dpopsuev/battery/mcpserver"
	"github.com/dpopsuev/battery/server"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "emcee"
	serverVersion = "0.8.0"

	defaultListMax   = 50
	defaultSearchMax = 20
)

var (
	errRefRequired     = errors.New("ref is required")
	errTitleRequired   = errors.New("title is required")
	errQueryRequired   = errors.New("query is required")
	errIssuesRequired  = errors.New("issues is required")
	errBodyRequired    = errors.New("body is required")
	errStageIDRequired = errors.New("stage_id is required")
	errNameRequired    = errors.New("name is required")
	errBackendNotFound = errors.New("backend not found")
	errIDRequired      = errors.New("id is required")
	errUnknownAction   = errors.New("unknown action")
)

// EmceeService combines all driver port interfaces.
type EmceeService interface {
	driver.IssueService
	driver.DocumentService
	driver.ProjectService
	driver.InitiativeService
	driver.LabelService
	driver.BulkService
	driver.HealthService
	driver.CommentService
	driver.StageService
	driver.LaunchService
	driver.BuildService
	driver.PipelineService
	driver.BackendManager
	driver.TriageService
	driver.FieldService
	driver.JQLService
	driver.PRService
}

const serverInstructions = `Emcee — All Ceremonies in one place. Unified issue tracker across Linear, GitHub, GitLab, Jira, and Report Portal. Ref format: "backend:key" (e.g. "jira:PROJ-42"). Backend is required for list/create/search actions.

## emcee tool — actions and required params:

Issues:
  list        — backend, [status, assignee, limit]
  get         — ref → returns issue with comments inline
  create      — backend, title, [description, status, priority, assignee, parent_id, project_id, issue_type, versions, fix_versions, components]
  update      — ref, [title, description, status, priority, assignee, components, fix_versions, resolution]
  search      — backend, query, [limit]
  children    — ref

Bulk:
  bulk_create — backend, issues (JSON array of create inputs)
  bulk_update — backend, issues (JSON array of {ref, title?, status?, priority?})

Comments:
  comments    — ref → list comments on an issue
  comment_add — ref, body → add comment without overwriting description

Staging (pre-submission cache, all backends):
  stage       — backend, title, [all create fields] → returns stage_id
  stage_list  — (no params) → list all staged items
  stage_show  — stage_id → preview staged payload
  stage_patch — stage_id, [title, description, status, priority, assignee]
  stage_drop  — stage_id → discard staged item
  push        — stage_id → submit to backend, re-stages on failure
  push_all    — (no params) → push all staged items to their backends

Report Portal:
  launches    — backend=reportportal, [query (name filter), status, limit]
  launch_get  — backend=reportportal, ref (launch ID)
  test_items  — backend=reportportal, ref (launch ID), [status, limit]
  test_item_get — backend=reportportal, ref (item ID)
  defect_update — backend=reportportal, issues (JSON array of {test_item_id, issue_type, comment?})

Jenkins CI:
  jobs         — backend=jenkins, [limit]
  job_get      — backend=jenkins, query (job name)
  build_trigger — backend=jenkins, query (job name), [issues (JSON object of build params)]
  build_get    — backend=jenkins, query (job name), ref (build number)
  build_log    — backend=jenkins, query (job name), ref (build number)
  test_results — backend=jenkins, query (job name), ref (build number)
  queue        — backend=jenkins
  builds       — backend=jenkins, query (job name), [limit]
  build_last   — backend=jenkins, query (job name)
  build_last_failed — backend=jenkins, query (job name)
  build_last_successful — backend=jenkins, query (job name)
  build_stop   — backend=jenkins, query (job name), ref (build number)
  job_params   — backend=jenkins, query (job name) → list parameter definitions
  folder_jobs  — backend=jenkins, query (folder path) → list jobs in folder
  job_upstream — backend=jenkins, query (job name) → upstream dependencies
  job_downstream — backend=jenkins, query (job name) → downstream dependencies
  build_artifacts — backend=jenkins, query (job name), ref (build number) → list build artifacts
  build_revision — backend=jenkins, query (job name), ref (build number) → get SCM revision
  build_causes — backend=jenkins, query (job name), ref (build number) → get build trigger causes
  nodes       — backend=jenkins → list all build agents
  node_get    — backend=jenkins, query (node name) → get node details
  views       — backend=jenkins → list all views
  view_jobs   — backend=jenkins, query (view name) → list jobs in view

Jenkins Pipelines:
  pipeline_runs — backend=jenkins, query (job name) → list pipeline runs
  pipeline_run_get — backend=jenkins, query (job name), ref (run ID) → get pipeline run details
  pipeline_inputs — backend=jenkins, query (job name), ref (run ID) → list pending input actions
  pipeline_input_approve — backend=jenkins, query (job name), ref (run ID) → approve pending input
  pipeline_input_abort — backend=jenkins, query (job name), ref (run ID) → abort pending input

Triage (Defect Lifecycle):
  triage       — ref (seed artifact, e.g. jira:OCPBUGS-123), [limit (max depth, default 3)] → cross-backend correlation graph

Pull Requests / Merge Requests:
  prs         — backend, [author, status, merged_after (YYYY-MM-DD), merged_before (YYYY-MM-DD), repo (override: owner/repo or namespace/project), limit]

Discovery:
  fields      — backend → list available fields (Jira: custom field IDs)
  jql         — backend=jira, query (raw JQL string), [limit]

## emcee_manage tool — supporting entities:
  doc_list, doc_create, project_list, project_create, project_update,
  initiative_list, initiative_create, label_list, label_create
  All take: action, backend, + entity-specific params.

## Enums:
  status: backlog, todo, in_progress, in_review, done, canceled
  priority: urgent, high, medium, low
  backends: linear, github, gitlab, jira, reportportal, jenkins
  backend names can be instance names (e.g. jenkins-ci, jira-prod) when configured via config.yaml with type: field
  backend is required — no default

## Notes:
  - create auto-stages on failure — error includes stage_id for retry
  - get returns comments inline (no separate call needed)
  - responses are cached with TTL; repeated reads are fast
  - versions, fix_versions, components are comma-separated strings (Jira)
  - issue_type defaults to "Task" on Jira if not specified`

// Serve starts the MCP server over stdio, exposing issue management tools.
func Serve(svc EmceeService) error {
	srv := mcpserver.NewServer(serverName, serverVersion).
		WithInstructions(serverInstructions)
	RegisterTools(srv, svc)
	return srv.Serve(context.Background(), &sdkmcp.StdioTransport{})
}

// RegisterTools registers all emcee MCP tools on the given server.
func RegisterTools(srv *mcpserver.Server, svc EmceeService) {
	srv.ToolWithSchema(
		server.ToolMeta{
			Name:        "emcee",
			Description: "Issue management across all backends. Actions: list, get, create, update, search, children, bulk_create, bulk_update, comments, comment_add, stage, stage_list, stage_show, stage_patch, stage_drop, push, push_all, launches, launch_get, test_items, test_item_get, defect_update, jobs, job_get, build_trigger, build_get, build_log, test_results, queue, build_artifacts, build_revision, build_causes, nodes, node_get, views, view_jobs, pipeline_runs, pipeline_run_get, pipeline_inputs, pipeline_input_approve, pipeline_input_abort, fields, jql, prs.",
			Keywords:    []string{"issue", "ticket", "bug", "task", "comment", "stage", "push", "linear", "github", "jira", "gitlab"},
			Categories:  []string{"issue-management"},
		},
		emceeSchema,
		emceeHandler(svc),
	)
	srv.ToolWithSchema(
		server.ToolMeta{
			Name:        "emcee_manage",
			Description: "Supporting entities and backend management. Actions: doc_list, doc_create, project_list, project_create, project_update, initiative_list, initiative_create, label_list, label_create, config_reload, backend_remove. All take action + backend + entity-specific params.",
			Keywords:    []string{"document", "project", "initiative", "label", "epic"},
			Categories:  []string{"issue-management", "project-management"},
		},
		manageSchema,
		manageHandler(svc),
	)
	srv.Tool(
		server.ToolMeta{
			Name:        "emcee_health",
			Description: "Check emcee backend health and configuration status",
			Keywords:    []string{"health", "status", "diagnostics"},
			Categories:  []string{"operations"},
		},
		healthHandler(svc),
	)
}

// --- Schemas ---

var emceeSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":      {"type": "string", "enum": ["list","get","create","update","search","children","bulk_create","bulk_update","comments","comment_add","stage","stage_list","stage_show","stage_patch","stage_drop","push","push_all","launches","launch_get","test_items","test_item_get","defect_update","jobs","job_get","build_trigger","build_get","build_log","test_results","queue","builds","build_last","build_last_failed","build_last_successful","build_stop","job_params","folder_jobs","job_upstream","job_downstream","build_artifacts","build_revision","build_causes","nodes","node_get","views","view_jobs","pipeline_runs","pipeline_run_get","pipeline_inputs","pipeline_input_approve","pipeline_input_abort","triage","fields","jql","prs"], "description": "Action to perform"},
		"backend":     {"type": "string", "description": "Backend name (required for list/create/search)"},
		"ref":         {"type": "string", "description": "Issue ref for get/update/children (e.g. linear:PROJ-42)"},
		"title":       {"type": "string", "description": "Issue title (create)"},
		"description": {"type": "string", "description": "Issue description (create/update)"},
		"status":      {"type": "string", "description": "Status: backlog, todo, in_progress, in_review, done, canceled"},
		"priority":    {"type": "string", "description": "Priority: urgent, high, medium, low"},
		"assignee":    {"type": "string", "description": "Assignee name (create/list filter)"},
		"parent_id":   {"type": "string", "description": "Parent issue ID for sub-issues (create)"},
		"project_id":  {"type": "string", "description": "Project ID (create)"},
		"query":       {"type": "string", "description": "Search query text (search)"},
		"limit":       {"type": "number", "description": "Max results (list/search)"},
		"issues":      {"type": "string", "description": "JSON array for bulk_create/bulk_update"},
		"body":        {"type": "string", "description": "Comment body text (comment_add)"},
		"stage_id":    {"type": "string", "description": "Stage ID for stage_show/stage_patch/stage_drop/push"},
		"issue_type":  {"type": "string", "description": "Issue type (create): Bug, Task, Story, Spike, etc. (Jira)"},
		"versions":    {"type": "string", "description": "Comma-separated affected versions (create, Jira): e.g. 4.16,4.17"},
		"fix_versions":{"type": "string", "description": "Comma-separated fix versions (create, Jira): e.g. 4.16.60"},
		"components":  {"type": "string", "description": "Comma-separated components (create, Jira)"},
		"author":      {"type": "string", "description": "Author username (prs filter)"},
		"merged_after": {"type": "string", "description": "Date filter for PRs (YYYY-MM-DD)"},
		"merged_before":{"type": "string", "description": "Date filter for PRs (YYYY-MM-DD)"},
		"repo":         {"type": "string", "description": "Repository override for PRs (e.g. owner/repo for GitHub, namespace/project for GitLab)"},
		"resolution":   {"type": "string", "description": "Resolution when closing (Jira): Done, Won't Fix, Duplicate, Cannot Reproduce, etc."}
	},
	"required": ["action"]
}`)

var manageSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":      {"type": "string", "enum": ["doc_list","doc_create","project_list","project_create","project_update","initiative_list","initiative_create","label_list","label_create","config_reload","backend_remove"], "description": "Action to perform"},
		"backend":     {"type": "string", "description": "Backend name (required for list/create/search)"},
		"title":       {"type": "string", "description": "Document title (doc_create)"},
		"name":        {"type": "string", "description": "Entity name (project/initiative/label create)"},
		"description": {"type": "string", "description": "Description (doc/project/initiative create)"},
		"content":     {"type": "string", "description": "Markdown content (doc_create)"},
		"project_id":  {"type": "string", "description": "Link document to project (doc_create)"},
		"id":          {"type": "string", "description": "Entity ID (project_update)"},
		"color":       {"type": "string", "description": "Label color hex (label_create)"},
		"limit":       {"type": "number", "description": "Max results (list actions)"}
	},
	"required": ["action"]
}`)

// --- Handlers ---

type emceeArgs struct {
	Action      string  `json:"action"`
	Backend     string  `json:"backend"`
	Ref         string  `json:"ref"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Status      string  `json:"status"`
	Priority    string  `json:"priority"`
	Assignee    string  `json:"assignee"`
	ParentID    string  `json:"parent_id"`
	ProjectID   string  `json:"project_id"`
	Query       string  `json:"query"`
	Limit       float64 `json:"limit"`
	Issues      string  `json:"issues"`
	Body        string  `json:"body"`
	StageID     string  `json:"stage_id"`
	IssueType   string  `json:"issue_type"`
	Author      string  `json:"author"`
	MergedAfter string  `json:"merged_after"`
	MergedBefore string `json:"merged_before"`
	Repo         string `json:"repo"`
	Versions    string  `json:"versions"`
	FixVersionsStr string `json:"fix_versions"`
	ComponentsStr  string `json:"components"`
	Resolution     string `json:"resolution"`
}

//nolint:gocyclo,funlen // dispatcher with many action cases
func emceeHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args emceeArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		limit := int(args.Limit)
		if limit == 0 {
			limit = defaultListMax
		}

		switch args.Action {
		case "list":
			filter := domain.ListFilter{
				Status:   domain.Status(args.Status),
				Assignee: args.Assignee,
				Limit:    limit,
			}
			issues, err := svc.List(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issues)

		case "get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			issue, err := svc.Get(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issue)

		case "create":
			if args.Title == "" {
				return "", errTitleRequired
			}
			input := domain.CreateInput{
				Title:       args.Title,
				Description: args.Description,
				Priority:    domain.ParsePriority(args.Priority),
				Assignee:    args.Assignee,
				ParentID:    args.ParentID,
				ProjectID:   args.ProjectID,
				IssueType:   args.IssueType,
				Versions:    splitCSV(args.Versions),
				FixVersions: splitCSV(args.FixVersionsStr),
				Components:  splitCSV(args.ComponentsStr),
			}
			if args.Status != "" {
				input.Status = domain.Status(args.Status)
			}
			issue, err := svc.Create(ctx, args.Backend, input)
			if err != nil {
				// Auto-stage on failure — preserve the payload for retry
				id := svc.StageItem(args.Backend, input, err.Error())
				return server.JSONResult(map[string]any{
					"error":    err.Error(),
					"staged":   true,
					"stage_id": id,
					"message":  fmt.Sprintf("Create failed, auto-staged as %s. Use push to retry.", id),
				})
			}
			return server.JSONResult(issue)

		case "update":
			if args.Ref == "" {
				return "", errRefRequired
			}
			var updateInput domain.UpdateInput
			if args.Title != "" {
				updateInput.Title = &args.Title
			}
			if args.Description != "" {
				updateInput.Description = &args.Description
			}
			if args.Status != "" {
				s := domain.Status(args.Status)
				updateInput.Status = &s
			}
			if args.Priority != "" {
				p := domain.ParsePriority(args.Priority)
				updateInput.Priority = &p
			}
			if args.ComponentsStr != "" {
				updateInput.Components = splitCSV(args.ComponentsStr)
			}
			if args.FixVersionsStr != "" {
				updateInput.FixVersions = splitCSV(args.FixVersionsStr)
			}
			if args.Resolution != "" {
				updateInput.Resolution = &args.Resolution
			}
			issue, err := svc.Update(ctx, args.Ref, updateInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issue)

		case "search":
			if args.Query == "" {
				return "", errQueryRequired
			}
			searchLimit := int(args.Limit)
			if searchLimit == 0 {
				searchLimit = defaultSearchMax
			}
			issues, err := svc.Search(ctx, args.Backend, args.Query, searchLimit)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issues)

		case "children":
			if args.Ref == "" {
				return "", errRefRequired
			}
			issues, err := svc.ListChildren(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issues)

		case "bulk_create":
			if args.Issues == "" {
				return "", errIssuesRequired
			}
			var inputs []domain.CreateInput
			if err := json.Unmarshal([]byte(args.Issues), &inputs); err != nil {
				return "", fmt.Errorf("invalid issues JSON: %w", err)
			}
			result, err := svc.BulkCreateIssues(ctx, args.Backend, inputs)
			if err != nil {
				return "", err
			}
			return server.JSONResult(result)

		case "bulk_update":
			if args.Issues == "" {
				return "", errIssuesRequired
			}
			var inputs []domain.BulkUpdateInput
			if err := json.Unmarshal([]byte(args.Issues), &inputs); err != nil {
				return "", fmt.Errorf("invalid issues JSON: %w", err)
			}
			result, err := svc.BulkUpdateIssues(ctx, args.Backend, inputs)
			if err != nil {
				return "", err
			}
			return server.JSONResult(result)

		// --- Comment actions ---

		case "comments":
			if args.Ref == "" {
				return "", errRefRequired
			}
			comments, err := svc.ListComments(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(comments)

		case "comment_add":
			if args.Ref == "" {
				return "", errRefRequired
			}
			if args.Body == "" {
				return "", errBodyRequired
			}
			comment, err := svc.AddComment(ctx, args.Ref, domain.CommentCreateInput{Body: args.Body})
			if err != nil {
				return "", err
			}
			return server.JSONResult(comment)

		// --- Stage actions ---

		case "stage":
			if args.Title == "" {
				return "", errTitleRequired
			}
			input := domain.CreateInput{
				Title:       args.Title,
				Description: args.Description,
				Priority:    domain.ParsePriority(args.Priority),
				Assignee:    args.Assignee,
				ParentID:    args.ParentID,
				ProjectID:   args.ProjectID,
				IssueType:   args.IssueType,
				Versions:    splitCSV(args.Versions),
				FixVersions: splitCSV(args.FixVersionsStr),
				Components:  splitCSV(args.ComponentsStr),
			}
			if args.Status != "" {
				input.Status = domain.Status(args.Status)
			}
			id := svc.StageItem(args.Backend, input, "")
			return server.JSONResult(map[string]string{"stage_id": id, "backend": args.Backend})

		case "stage_list":
			items := svc.StageList()
			return server.JSONResult(items)

		case "stage_show":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			item, err := svc.StageGet(args.StageID)
			if err != nil {
				return "", err
			}
			return server.JSONResult(item)

		case "stage_patch":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			var patchInput domain.UpdateInput
			if args.Title != "" {
				patchInput.Title = &args.Title
			}
			if args.Description != "" {
				patchInput.Description = &args.Description
			}
			if args.Status != "" {
				s := domain.Status(args.Status)
				patchInput.Status = &s
			}
			if args.Priority != "" {
				p := domain.ParsePriority(args.Priority)
				patchInput.Priority = &p
			}
			if args.Assignee != "" {
				patchInput.Assignee = &args.Assignee
			}
			item, err := svc.StagePatch(args.StageID, patchInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(item)

		case "stage_drop":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			if err := svc.StageDrop(args.StageID); err != nil {
				return "", err
			}
			return server.JSONResult(map[string]string{"status": "dropped", "id": args.StageID})

		case "push":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			item, err := svc.StagePop(args.StageID)
			if err != nil {
				return "", err
			}
			issue, err := svc.Create(ctx, item.Backend, item.Input)
			if err != nil {
				svc.StageItem(item.Backend, item.Input, err.Error())
				return "", fmt.Errorf("push failed (re-staged): %w", err)
			}
			return server.JSONResult(issue)

		case "push_all":
			items := svc.StagePopAll()
			if len(items) == 0 {
				return server.JSONResult(map[string]any{"pushed": 0, "errors": []string{}})
			}
			var pushed []domain.Issue
			var pushErrs []string
			for i := range items {
				issue, err := svc.Create(ctx, items[i].Backend, items[i].Input)
				if err != nil {
					svc.StageItem(items[i].Backend, items[i].Input, err.Error())
					pushErrs = append(pushErrs, fmt.Sprintf("%s: %v", items[i].ID, err))
					continue
				}
				pushed = append(pushed, *issue)
			}
			return server.JSONResult(map[string]any{"pushed": len(pushed), "issues": pushed, "errors": pushErrs})

		// --- Launch/RP actions ---

		case "launches":
			filter := domain.LaunchFilter{
				Name:   args.Query,
				Status: args.Status,
				Limit:  int(args.Limit),
			}
			launches, err := svc.ListLaunches(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(launches)

		case "launch_get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			launch, err := svc.GetLaunch(ctx, args.Backend, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(launch)

		case "test_items":
			if args.Ref == "" {
				return "", errRefRequired
			}
			filter := domain.TestItemFilter{
				Status: args.Status,
				Limit:  int(args.Limit),
			}
			items, err := svc.ListTestItems(ctx, args.Backend, args.Ref, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(items)

		case "test_item_get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			item, err := svc.GetTestItem(ctx, args.Backend, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(item)

		case "defect_update":
			if args.Issues == "" {
				return "", errIssuesRequired
			}
			var updates []domain.DefectUpdate
			if err := json.Unmarshal([]byte(args.Issues), &updates); err != nil {
				return "", fmt.Errorf("invalid defect updates JSON: %w", err)
			}
			if err := svc.UpdateDefects(ctx, args.Backend, updates); err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"updated": len(updates)})

		// --- Build/Jenkins actions ---

		case "jobs":
			filter := domain.JobFilter{Limit: int(args.Limit)}
			jobs, err := svc.ListJobs(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(jobs)

		case "job_get":
			if args.Query == "" {
				return "", errQueryRequired
			}
			job, err := svc.GetJob(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(job)

		case "build_trigger":
			if args.Query == "" {
				return "", errQueryRequired
			}
			var params map[string]string
			if args.Issues != "" {
				if err := json.Unmarshal([]byte(args.Issues), &params); err != nil {
					return "", fmt.Errorf("invalid build params JSON: %w", err)
				}
			}
			queueID, err := svc.TriggerBuild(ctx, args.Backend, args.Query, params)
			if err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"queue_id": queueID, "job": args.Query})

		case "build_get":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			build, err := svc.GetBuild(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(build)

		case "build_log":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			log, err := svc.GetBuildLog(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(map[string]string{"log": log})

		case "test_results":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			results, err := svc.GetTestResults(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(results)

		case "builds":
			if args.Query == "" {
				return "", errQueryRequired
			}
			limit := int(args.Limit)
			if limit == 0 {
				limit = defaultListMax
			}
			builds, err := svc.ListBuilds(ctx, args.Backend, args.Query, limit)
			if err != nil {
				return "", err
			}
			return server.JSONResult(builds)

		case "build_last":
			if args.Query == "" {
				return "", errQueryRequired
			}
			build, err := svc.GetLastBuild(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(build)

		case "build_last_failed":
			if args.Query == "" {
				return "", errQueryRequired
			}
			build, err := svc.GetLastFailedBuild(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(build)

		case "build_last_successful":
			if args.Query == "" {
				return "", errQueryRequired
			}
			build, err := svc.GetLastSuccessfulBuild(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(build)

		case "build_stop":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			if err := svc.StopBuild(ctx, args.Backend, args.Query, number); err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"stopped": true, "job": args.Query, "number": number})

		case "job_params":
			if args.Query == "" {
				return "", errQueryRequired
			}
			params, err := svc.GetJobParameters(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(params)

		case "folder_jobs":
			if args.Query == "" {
				return "", errQueryRequired
			}
			jobs, err := svc.ListFolderJobs(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(jobs)

		case "job_upstream":
			if args.Query == "" {
				return "", errQueryRequired
			}
			jobs, err := svc.GetUpstreamJobs(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(jobs)

		case "job_downstream":
			if args.Query == "" {
				return "", errQueryRequired
			}
			jobs, err := svc.GetDownstreamJobs(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(jobs)

		case "build_artifacts":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			artifacts, err := svc.ListArtifacts(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(artifacts)

		case "build_revision":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			rev, err := svc.GetBuildRevision(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(map[string]string{"revision": rev})

		case "build_causes":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			number, err := strconv.ParseInt(args.Ref, 10, 64)
			if err != nil {
				return "", fmt.Errorf("invalid build number %q: %w", args.Ref, err)
			}
			causes, err := svc.GetBuildCauses(ctx, args.Backend, args.Query, number)
			if err != nil {
				return "", err
			}
			return server.JSONResult(causes)

		case "nodes":
			nodes, err := svc.ListNodes(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONResult(nodes)

		case "node_get":
			if args.Query == "" {
				return "", errQueryRequired
			}
			node, err := svc.GetNode(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(node)

		case "views":
			views, err := svc.ListViews(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONResult(views)

		case "view_jobs":
			if args.Query == "" {
				return "", errQueryRequired
			}
			jobs, err := svc.GetViewJobs(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(jobs)

		case "queue":
			items, err := svc.GetQueue(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONResult(items)

		// --- Pipeline actions ---

		case "pipeline_runs":
			if args.Query == "" {
				return "", errQueryRequired
			}
			runs, err := svc.ListPipelineRuns(ctx, args.Backend, args.Query)
			if err != nil {
				return "", err
			}
			return server.JSONResult(runs)

		case "pipeline_run_get":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			run, err := svc.GetPipelineRun(ctx, args.Backend, args.Query, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(run)

		case "pipeline_inputs":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			inputs, err := svc.GetPendingInputs(ctx, args.Backend, args.Query, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONResult(inputs)

		case "pipeline_input_approve":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			if err := svc.ApproveInput(ctx, args.Backend, args.Query, args.Ref); err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"approved": true, "job": args.Query, "run_id": args.Ref})

		case "pipeline_input_abort":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref == "" {
				return "", errRefRequired
			}
			if err := svc.AbortInput(ctx, args.Backend, args.Query, args.Ref); err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"aborted": true, "job": args.Query, "run_id": args.Ref})

		// --- Triage ---

		case "triage":
			if args.Ref == "" {
				return "", errRefRequired
			}
			depth := int(args.Limit)
			if depth == 0 {
				depth = 3
			}
			graph, err := svc.Triage(ctx, args.Ref, depth)
			if err != nil {
				return "", err
			}
			return server.JSONResult(graph)

		// --- Field discovery + JQL ---

		case "fields":
			fields, err := svc.ListFields(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONResult(fields)

		case "jql":
			if args.Query == "" {
				return "", errQueryRequired
			}
			jqlLimit := int(args.Limit)
			if jqlLimit == 0 {
				jqlLimit = defaultListMax
			}
			issues, err := svc.SearchJQL(ctx, args.Backend, args.Query, jqlLimit)
			if err != nil {
				return "", err
			}
			return server.JSONResult(issues)

		case "prs":
			filter := domain.PRFilter{
				Author:       args.Author,
				State:        args.Status,
				MergedAfter:  args.MergedAfter,
				MergedBefore: args.MergedBefore,
				Repo:         args.Repo,
				Limit:        int(args.Limit),
			}
			prs, err := svc.ListPRs(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(prs)

		default:
			return "", fmt.Errorf("%w %q", errUnknownAction, args.Action)
		}
	}
}

type manageArgs struct {
	Action      string  `json:"action"`
	Backend     string  `json:"backend"`
	Title       string  `json:"title"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Content     string  `json:"content"`
	ProjectID   string  `json:"project_id"`
	ID          string  `json:"id"`
	Color       string  `json:"color"`
	Limit       float64 `json:"limit"`
}

//nolint:gocyclo,funlen // dispatcher with many action cases
func manageHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args manageArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		limit := int(args.Limit)
		if limit == 0 {
			limit = defaultListMax
		}

		switch args.Action {
		case "doc_list":
			filter := domain.DocumentListFilter{Limit: limit}
			docs, err := svc.ListDocuments(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(docs)

		case "doc_create":
			if args.Title == "" {
				return "", errTitleRequired
			}
			docInput := domain.DocumentCreateInput{
				Title:     args.Title,
				Content:   args.Content,
				ProjectID: args.ProjectID,
			}
			doc, err := svc.CreateDocument(ctx, args.Backend, docInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(doc)

		case "project_list":
			filter := domain.ProjectListFilter{Limit: limit}
			projects, err := svc.ListProjects(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(projects)

		case "project_create":
			if args.Name == "" {
				return "", errNameRequired
			}
			projInput := domain.ProjectCreateInput{
				Name:        args.Name,
				Description: args.Description,
			}
			proj, err := svc.CreateProject(ctx, args.Backend, projInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(proj)

		case "project_update":
			if args.ID == "" {
				return "", errIDRequired
			}
			var projUpdate domain.ProjectUpdateInput
			if args.Name != "" {
				projUpdate.Name = &args.Name
			}
			if args.Description != "" {
				projUpdate.Description = &args.Description
			}
			proj, err := svc.UpdateProject(ctx, args.Backend, args.ID, projUpdate)
			if err != nil {
				return "", err
			}
			return server.JSONResult(proj)

		case "initiative_list":
			filter := domain.InitiativeListFilter{Limit: limit}
			inits, err := svc.ListInitiatives(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONResult(inits)

		case "initiative_create":
			if args.Name == "" {
				return "", errNameRequired
			}
			initInput := domain.InitiativeCreateInput{
				Name:        args.Name,
				Description: args.Description,
			}
			init, err := svc.CreateInitiative(ctx, args.Backend, initInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(init)

		case "label_list":
			labels, err := svc.ListLabels(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONResult(labels)

		case "label_create":
			if args.Name == "" {
				return "", errNameRequired
			}
			labelInput := domain.LabelCreateInput{
				Name:  args.Name,
				Color: args.Color,
			}
			label, err := svc.CreateLabel(ctx, args.Backend, labelInput)
			if err != nil {
				return "", err
			}
			return server.JSONResult(label)

		case "config_reload":
			added, removed, err := svc.ReloadConfig("")
			if err != nil {
				return "", err
			}
			return server.JSONResult(map[string]any{"added": added, "removed": removed})

		case "backend_remove":
			if args.Name == "" {
				return "", errNameRequired
			}
			ok := svc.RemoveBackend(args.Name)
			if !ok {
				return "", fmt.Errorf("%w: %s", errBackendNotFound, args.Name)
			}
			return server.JSONResult(map[string]string{"removed": args.Name})

		default:
			return "", fmt.Errorf("%w %q (valid: doc_list, doc_create, project_list, project_create, project_update, initiative_list, initiative_create, label_list, label_create, config_reload, backend_remove)", errUnknownAction, args.Action)
		}
	}
}

func healthHandler(svc EmceeService) server.Handler {
	return func(_ context.Context, _ json.RawMessage) (string, error) {
		health := svc.Health()
		return server.JSONResult(health)
	}
}

// splitCSV splits a comma-separated string into a trimmed slice.
// Returns nil for empty input.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
