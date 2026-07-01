// Package mcp implements the driver (inbound) adapter as an MCP stdio server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	batterymcp "github.com/dpopsuev/battery/mcp"
	"github.com/dpopsuev/battery/server"
	"github.com/dpopsuev/emcee/internal/config"
	"github.com/dpopsuev/emcee/internal/docparse"
	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/service"
	"github.com/dpopsuev/emcee/internal/timeexpr"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	serverName    = "emcee"
	serverVersion = "0.15.0"

	defaultListMax   = 50
	defaultSearchMax = 20
)

var (
	errRefRequired       = errors.New("ref is required")
	errTitleRequired     = errors.New("title is required")
	errQueryRequired     = errors.New("query is required")
	errIssuesRequired    = errors.New("issues is required")
	errBodyRequired      = errors.New("body is required")
	errStageIDRequired   = errors.New("stage_id is required")
	errTargetRefRequired = errors.New("target_ref is required for link_issue")
	errNameRequired      = errors.New("name is required")
	errBackendNotFound   = errors.New("backend not found")
	errBackendRequired   = errors.New("backend is required")
	errIDRequired        = errors.New("id is required")
	errUnknownAction     = errors.New("unknown action")
	errFieldRequired     = errors.New("field is required for view_mutate")
	errProjectIDRequired = errors.New("project_id is required")
)

// EmceeService combines all driver port interfaces.
type EmceeService interface {
	service.IssueService
	service.DocumentService
	service.ProjectService
	service.InitiativeService
	service.LabelService
	service.BulkService
	service.HealthService
	service.CommentService
	service.LaunchService
	service.StageService
	service.BackendManager
	service.TriageService
	service.IssueLinkService
	service.GistService
	service.PRReviewService
	service.ChangelogService
	service.FieldService
	service.TemplateService
	service.JQLService
	service.PRService
	service.LedgerService
	service.ViewService
	service.ProjectScopeService
}

const serverInstructions = "Emcee — issue tracking, test analytics, and knowledge management. " +
	"5 tools: issue (CRUD), view (local cache), launch (Report Portal), doc (documents), admin (meta/triage/ledger). " +
	"Start with admin(action=help). Ref format: backend:key (e.g. jira:PROJ-42). " +
	"Typical: issue(list) → view(pull) → view(mutate) → view(push). " +
	"For RP: launch(pull, ref=37337) → launch(items, status=FAILED) → launch(defect_update)."

// Serve starts the MCP server over stdio, exposing issue management tools.
func Serve(svc EmceeService) error {
	srv := batterymcp.NewServer(serverName, serverVersion).
		WithInstructions(serverInstructions)
	RegisterTools(srv, svc)
	return srv.Serve(context.Background(), &sdkmcp.StdioTransport{})
}

// ServeHTTP starts the MCP server over HTTP on the given address (e.g. ":8080").
// POST /mcp — stateless StreamableHTTP transport (MCP 2025-03-26)
// GET  /health — backend health as JSON
func ServeHTTP(addr string, svc EmceeService) error {
	srv := batterymcp.NewServer(serverName, serverVersion).
		WithInstructions(serverInstructions).
		WithInitTimeout(0)
	RegisterTools(srv, svc)

	mcpHandler := sdkmcp.NewStreamableHTTPHandler(
		func(*http.Request) *sdkmcp.Server { return srv.SDK() },
		&sdkmcp.StreamableHTTPOptions{Stateless: true},
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", mcpHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		status := svc.Health()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(status)
	})

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 30 * time.Second,
	}
	slog.Info("emcee MCP server listening", slog.String("addr", addr))
	return httpSrv.ListenAndServe()
}

// RegisterTools registers all emcee MCP tools on the given server.
func RegisterTools(srv *batterymcp.Server, svc EmceeService) {
	srv.ToolStringHandler(server.ToolMeta{
		Name:        "issue",
		Description: "Issue CRUD across all backends. list | get | create | update | search | children | bulk_create | bulk_update | comments | comment_add | link | unlink | link_types | stage | stage_list | stage_show | stage_patch | stage_drop | push | push_all | fields | jql | prs | pr_reviews | pr_comments | changelog",
		Keywords:    []string{"issue", "ticket", "bug", "task", "jira", "linear", "github", "gitlab", "comment", "stage", "push", "jql", "pr"},
		Categories:  []string{"issue-management"},
	}, issueSchema, issueHandler(svc))

	srv.ToolStringHandler(server.ToolMeta{
		Name:        "view",
		Description: "Local materialized view (Identity Map + Unit of Work). pull | get | mutate | diff | push | push_all | list | dirty | drop | reset. Works for issue refs (jira:KEY) and launch refs (reportportal:ID).",
		Keywords:    []string{"view", "pull", "local", "cache", "mutate", "diff", "push"},
		Categories:  []string{"issue-management"},
	}, viewSchema, viewHandler(svc))

	srv.ToolStringHandler(server.ToolMeta{
		Name:        "launch",
		Description: "Report Portal launches, test items, defects, dashboards. pull | list | get | items | search_items | item_get | bulk_item_get | defect_update | dashboards | dashboard_get | dashboard_create | widget_add",
		Keywords:    []string{"launch", "test", "reportportal", "defect", "ci", "dashboard"},
		Categories:  []string{"test-analytics"},
	}, launchSchema, launchHandler(svc))

	srv.ToolStringHandler(server.ToolMeta{
		Name:        "doc",
		Description: "Document operations. parse | links | diff | audit | terms | validate | declarations | sync_gist | sync_jira",
		Keywords:    []string{"doc", "markdown", "parse", "gist", "sync", "validate"},
		Categories:  []string{"knowledge"},
	}, docSchema, docHandler(svc))

	srv.ToolStringHandler(server.ToolMeta{
		Name:        "admin",
		Description: "Meta: help | triage | triage_config | triage_config_set | changelog | fields_discover | ledger_list | ledger_get | ledger_search | ledger_similar | ledger_ingest | ledger_stats. Start here.",
		Keywords:    []string{"help", "triage", "ledger", "discover", "admin"},
		Categories:  []string{"operations"},
	}, adminSchema, adminHandler(svc))

	srv.ToolStringHandler(server.ToolMeta{
		Name:        "emcee_manage",
		Description: "Supporting entities: doc_list, doc_create, project_list, project_create, project_update, initiative_list, initiative_create, label_list, label_create, config_reload, backend_remove.",
		Keywords:    []string{"document", "project", "initiative", "label"},
		Categories:  []string{"project-management"},
	}, manageSchema, manageHandler(svc))

	srv.ToolString(server.ToolMeta{
		Name:        "emcee_health",
		Description: "Check emcee backend health and configuration status",
		Keywords:    []string{"health", "status"},
		Categories:  []string{"operations"},
	}, healthHandler(svc))
}

// --- Schemas ---

var issueSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":       {"type": "string", "enum": ["list","get","create","update","search","children","bulk_create","bulk_update","comments","comment_add","link","unlink","link_types","stage","stage_list","stage_show","stage_patch","stage_drop","push","push_all","fields","jql","prs","pr_reviews","pr_comments","changelog","set_default_project"], "description": "Action to perform"},
		"backend":      {"type": "string", "description": "Backend name (list/create/search/jql/prs/fields)"},
		"ref":          {"type": "string", "description": "Issue ref e.g. jira:PROJ-42"},
		"title":        {"type": "string", "description": "Issue title (create/stage)"},
		"description":  {"type": "string", "description": "Issue description"},
		"status":       {"type": "string", "description": "backlog | todo | in_progress | in_review | done | canceled"},
		"priority":     {"type": "string", "description": "urgent | high | medium | low"},
		"assignee":     {"type": "string", "description": "Assignee name"},
		"parent_id":    {"type": "string", "description": "Parent issue ID (create)"},
		"project_id":   {"type": "string", "description": "Project ID (create)"},
		"issue_type":   {"type": "string", "description": "Issue type (Jira: Bug/Task/Story) or link type (link: Blocks/Relates)"},
		"target_ref":   {"type": "string", "description": "Target ref for link (e.g. jira:PROJ-2)"},
		"query":        {"type": "string", "description": "Search query or raw JQL string"},
		"limit":        {"type": "number", "description": "Max results"},
		"issues":       {"type": "string", "description": "JSON array for bulk_create / bulk_update"},
		"body":         {"type": "string", "description": "Comment body (comment_add)"},
		"stage_id":     {"type": "string", "description": "Stage ID (stage_show/patch/drop/push)"},
		"versions":     {"type": "string", "description": "Affected versions, comma-separated (Jira)"},
		"fix_versions": {"type": "string", "description": "Fix versions, comma-separated (Jira)"},
		"components":   {"type": "string", "description": "Components, comma-separated (Jira)"},
		"resolution":   {"type": "string", "description": "Resolution on close (Jira): Done, Won't Fix, Duplicate, Cannot Reproduce"},
		"author":       {"type": "string", "description": "Author filter (prs)"},
		"merged_after": {"type": "string", "description": "Date lower bound for prs (YYYY-MM-DD)"},
		"merged_before":{"type": "string", "description": "Date upper bound for prs (YYYY-MM-DD)"},
		"repo":         {"type": "string", "description": "Repo override for prs (owner/repo or namespace/project)"}
	},
	"required": ["action"]
}`)

var viewSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action": {"type": "string", "enum": ["pull","get","mutate","diff","push","push_all","list","dirty","drop","reset"], "description": "Action to perform"},
		"ref":    {"type": "string", "description": "Issue ref (jira:KEY) or launch ref (reportportal:ID)"},
		"field":  {"type": "string", "description": "Field name to mutate (mutate)"},
		"value":  {"type": "string", "description": "New field value (mutate)"},
		"body":   {"type": "string", "description": "Alias for value (mutate)"}
	},
	"required": ["action"]
}`)

var launchSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":       {"type": "string", "enum": ["pull","list","get","items","search_items","item_get","bulk_item_get","defect_update","dashboards","dashboard_get","dashboard_create","widget_add","tree"], "description": "Action. pull caches launch+items locally; subsequent items calls are cache-first."},
		"backend":      {"type": "string", "description": "Backend name (reportportal)"},
		"ref":          {"type": "string", "description": "Launch ID (pull/list/get/items) or item ID (item_get) or dashboard ID"},
		"query":        {"type": "string", "description": "Name filter (list) or dashboard description (dashboard_create)"},
		"status":       {"type": "string", "description": "FAILED | PASSED | SKIPPED — single value or comma-separated list (items/search_items)."},
		"since":        {"type": "string", "description": "Lower time bound. RFC3339, named anchor (startOfWeek, startOfDay, startOfMonth, now), or relative offset (-7d, -2w, -1h)."},
		"before":       {"type": "string", "description": "Upper time bound. RFC3339, named anchor (endOfWeek, endOfDay, endOfMonth, now), or relative offset (-1d, -1h)."},
		"ci_lane":      {"type": "string", "description": "Exact ci-lane attribute filter for search_items (e.g. 'telco-ft-ran-ptp'). Excludes gm/gnrd launches at source."},
		"issue_type":   {"type": "string", "description": "ti001 | pb001 | ab001 — single value or comma-separated list (search_items/defect_update)."},
		"limit":        {"type": "number", "description": "Max results or widget width"},
		"page":         {"type": "number", "description": "0-based page number"},
		"include_logs": {"type": "boolean", "description": "Fetch failure_message for FAILED items"},
		"issues":       {"type": "string", "description": "JSON array: defect_update=[{test_item_id,issue_type,comment?}] or bulk_item_get=[id,...]"},
		"title":        {"type": "string", "description": "Dashboard name or widget name"},
		"description":  {"type": "string", "description": "Dashboard description"},
		"issue_type":   {"type": "string", "description": "Widget type (widget_add)"}
	},
	"required": ["action"]
}`)

var docSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":   {"type": "string", "enum": ["parse","links","diff","audit","terms","validate","declarations","sync_gist","sync_jira"], "description": "Action to perform"},
		"query":    {"type": "string", "description": "Markdown content (parse/links/diff/audit/terms/validate/declarations/sync_gist/sync_jira)"},
		"body":     {"type": "string", "description": "New markdown for diff; comma-separated terms for terms; required section titles for validate"},
		"ref":      {"type": "string", "description": "Gist ID for sync_gist update; Jira ref for sync_jira (e.g. jira:KEY)"},
		"title":    {"type": "string", "description": "Gist filename (sync_gist)"},
		"backend":  {"type": "string", "description": "Backend name (sync_gist=github, sync_jira=jira)"}
	},
	"required": ["action"]
}`)

var adminSchema = json.RawMessage(`{
	"type": "object",
	"properties": {
		"action":      {"type": "string", "enum": ["help","triage","triage_config","triage_config_set","changelog","fields_discover","template_discover","ledger_list","ledger_get","ledger_search","ledger_similar","ledger_ingest","ledger_stats"], "description": "Action. Start with help."},
		"ref":         {"type": "string", "description": "Seed ref (triage/ledger_get/ledger_similar/ledger_ingest/changelog)"},
		"backend":     {"type": "string", "description": "Backend name (fields_discover/ledger_list/ledger_ingest)"},
		"query":       {"type": "string", "description": "Search query (ledger_search)"},
		"limit":       {"type": "number", "description": "Max results or triage depth (default 3)"},
		"issues":      {"type": "string", "description": "JSON allow-list of backend names (triage_config_set)"},
		"title":       {"type": "string", "description": "Artifact title (ledger_ingest)"},
		"description": {"type": "string", "description": "Artifact text (ledger_ingest)"},
		"status":      {"type": "string", "description": "Status filter (ledger_list/ledger_ingest)"},
		"issue_type":  {"type": "string", "description": "Type filter (ledger_list/ledger_ingest)"},
		"components":  {"type": "string", "description": "Components, comma-separated (ledger_list/ledger_ingest)"}
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
	Action         string  `json:"action"`
	Backend        string  `json:"backend"`
	Ref            string  `json:"ref"`
	Title          string  `json:"title"`
	Description    string  `json:"description"`
	Status         string  `json:"status"`
	Priority       string  `json:"priority"`
	Assignee       string  `json:"assignee"`
	ParentID       string  `json:"parent_id"`
	ProjectID      string  `json:"project_id"`
	Query          string  `json:"query"`
	Limit          float64 `json:"limit"`
	Issues         string  `json:"issues"`
	Body           string  `json:"body"`
	StageID        string  `json:"stage_id"`
	IssueType      string  `json:"issue_type"`
	Author         string  `json:"author"`
	MergedAfter    string  `json:"merged_after"`
	MergedBefore   string  `json:"merged_before"`
	Repo           string  `json:"repo"`
	Versions       string  `json:"versions"`
	FixVersionsStr string  `json:"fix_versions"`
	ComponentsStr  string  `json:"components"`
	Resolution     string  `json:"resolution"`
	Page           float64 `json:"page"`
	Since          string  `json:"since"`
	Before         string  `json:"before"`
	CILane         string  `json:"ci_lane"`
	IncludeLogs    bool    `json:"include_logs"`
	TargetRef      string  `json:"target_ref"`
	Field          string  `json:"field"`
	Value          string  `json:"value"`
}

//nolint:gocyclo,funlen // dispatcher with many action cases
//nolint:gocyclo,funlen // action dispatcher
func issueHandler(svc EmceeService) server.Handler {
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
			return server.JSONString(issues)

		case "get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			issue, err := svc.Get(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(issue)

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
				id := svc.StageItem(args.Backend, input, err.Error())
				return server.JSONString(map[string]any{
					"error":    err.Error(),
					"staged":   true,
					"stage_id": id,
					"message":  fmt.Sprintf("Create failed, auto-staged as %s. Use push to retry.", id),
				})
			}
			return server.JSONString(issueWithProjectNote(issue, args.ProjectID, svc.DefaultProject(args.Backend)))

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
			return server.JSONString(issue)

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
			return server.JSONString(issues)

		case "children":
			if args.Ref == "" {
				return "", errRefRequired
			}
			issues, err := svc.ListChildren(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(issues)

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
			return server.JSONString(result)

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
			return server.JSONString(result)

		// --- Comment actions ---

		case "comments":
			if args.Ref == "" {
				return "", errRefRequired
			}
			comments, err := svc.ListComments(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(comments)

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
			return server.JSONString(comment)

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
			return server.JSONString(map[string]string{"stage_id": id, "backend": args.Backend})

		case "stage_list":
			items := svc.StageList()
			return server.JSONString(items)

		case "stage_show":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			item, err := svc.StageGet(args.StageID)
			if err != nil {
				return "", err
			}
			return server.JSONString(item)

		case "stage_patch":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			var patchInput domain.StagePatchInput
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
			if args.ComponentsStr != "" {
				patchInput.Components = splitCSV(args.ComponentsStr)
			}
			if args.FixVersionsStr != "" {
				patchInput.FixVersions = splitCSV(args.FixVersionsStr)
			}
			if args.ProjectID != "" {
				patchInput.ProjectID = &args.ProjectID
			}
			if args.ParentID != "" {
				patchInput.ParentID = &args.ParentID
			}
			if args.IssueType != "" {
				patchInput.IssueType = &args.IssueType
			}
			if args.Versions != "" {
				patchInput.Versions = splitCSV(args.Versions)
			}
			item, err := svc.StagePatch(args.StageID, patchInput)
			if err != nil {
				return "", err
			}
			return server.JSONString(item)

		case "stage_drop":
			if args.StageID == "" {
				return "", errStageIDRequired
			}
			if err := svc.StageDrop(args.StageID); err != nil {
				return "", err
			}
			return server.JSONString(map[string]string{"status": "dropped", "id": args.StageID})

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
			return server.JSONString(issueWithProjectNote(issue, item.Input.ProjectID, svc.DefaultProject(item.Backend)))

		case "push_all":
			items := svc.StagePopAll()
			if len(items) == 0 {
				return server.JSONString(map[string]any{"pushed": 0, "errors": []string{}})
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
			return server.JSONString(map[string]any{"pushed": len(pushed), "issues": pushed, "errors": pushErrs})

		// --- Issue links ---

		case "link":
			if args.Ref == "" {
				return "", errRefRequired
			}
			// target_ref is the dedicated param; fall back to query for backward compat.
			outwardRaw := args.TargetRef
			if outwardRaw == "" {
				outwardRaw = args.Query
			}
			if outwardRaw == "" {
				return "", errTargetRefRequired
			}
			backend, inwardKey, err := parseRef(args.Ref)
			if err != nil {
				return "", err
			}
			_, outwardKey, err := parseRef(outwardRaw)
			if err != nil {
				outwardKey = outwardRaw // bare key like "CNF-24028"
			}
			input := domain.IssueLinkInput{
				Type:       args.IssueType,
				InwardKey:  inwardKey,
				OutwardKey: outwardKey,
			}
			if err := svc.LinkIssue(ctx, backend, input); err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{"linked": true, "type": input.Type, "inward": inwardKey, "outward": outwardKey})

		case "unlink":
			if args.Ref == "" {
				return "", errRefRequired
			}
			if args.TargetRef == "" {
				return "", errTargetRefRequired
			}
			backend, inwardKey, err := parseRef(args.Ref)
			if err != nil {
				return "", err
			}
			_, outwardKey, err := parseRef(args.TargetRef)
			if err != nil {
				outwardKey = args.TargetRef
			}
			if err := svc.UnlinkIssue(ctx, backend, inwardKey, outwardKey, args.IssueType); err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{"unlinked": true, "inward": inwardKey, "outward": outwardKey})

		case "link_types":
			if args.Backend == "" {
				return "", errBackendRequired
			}
			types, err := svc.ListLinkTypes(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONString(types)

		// --- Report Portal ---

		case "pr_reviews":
			if args.Ref == "" {
				return "", errRefRequired
			}
			prNum, err := strconv.Atoi(args.Ref)
			if err != nil {
				return "", fmt.Errorf("invalid PR number %q: %w", args.Ref, err)
			}
			reviews, err := svc.ListPRReviews(ctx, args.Backend, prNum)
			if err != nil {
				return "", err
			}
			return server.JSONString(reviews)

		case "pr_comments":
			if args.Ref == "" {
				return "", errRefRequired
			}
			prNum, err := strconv.Atoi(args.Ref)
			if err != nil {
				return "", fmt.Errorf("invalid PR number %q: %w", args.Ref, err)
			}
			comments, err := svc.ListPRComments(ctx, args.Backend, prNum)
			if err != nil {
				return "", err
			}
			return server.JSONString(comments)

		// --- Doc sync ---

		case "changelog":
			if args.Ref == "" {
				return "", errRefRequired
			}
			entries, err := svc.ListChangelog(ctx, args.Ref, int(args.Limit))
			if err != nil {
				return "", err
			}
			return server.JSONString(entries)

		case "fields":
			fields, err := svc.ListFields(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONString(fields)

		case "fields_discover":
			if args.Backend == "" {
				return "", errBackendRequired
			}
			mappings, err := svc.DiscoverFields(ctx, args.Backend, config.Dir())
			if err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{
				"backend":  args.Backend,
				"manifest": config.DefaultPath(args.Backend),
				"mappings": mappings,
			})

		case "template_discover":
			if args.Backend == "" {
				return "", errBackendRequired
			}
			project := args.Ref
			if project == "" {
				return "", errRefRequired
			}
			issueType := args.IssueType
			if issueType == "" {
				issueType = "Bug"
			}
			limit := args.Limit
			if limit <= 0 {
				limit = 5
			}
			tmpl, err := svc.DiscoverTemplate(ctx, args.Backend, project, issueType, int(limit))
			if err != nil {
				return "", err
			}
			if tmpl == nil {
				return server.JSONString(map[string]any{"message": "no template found"})
			}
			return server.JSONString(tmpl)

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
			return server.JSONString(issues)

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
			return server.JSONString(prs)

		case "set_default_project":
			if args.Backend == "" {
				return "", errBackendRequired
			}
			if args.ProjectID == "" {
				return "", errProjectIDRequired
			}
			old := svc.DefaultProject(args.Backend)
			if err := svc.SetDefaultProject(args.Backend, args.ProjectID); err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{
				"backend":  args.Backend,
				"previous": old,
				"current":  args.ProjectID,
				"message":  fmt.Sprintf("Default project for %s changed: %s → %s", args.Backend, old, args.ProjectID),
			})

		// --- Ledger actions ---

		default:
			return "", fmt.Errorf("%w %q", errUnknownAction, args.Action)
		}
	}
}

//nolint:gocyclo,funlen // action dispatcher
func viewHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args emceeArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		_ = int(args.Limit) // limit unused in this handler

		switch args.Action {
		case "pull":
			if args.Ref == "" {
				return "", errRefRequired
			}
			vr, err := svc.ViewPull(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(vr)

		case "get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			vr, err := svc.ViewGet(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(vr)

		case "mutate":
			if args.Ref == "" {
				return "", errRefRequired
			}
			// Accept dedicated field/value params; fall back to query/body for compat.
			field := args.Field
			if field == "" {
				field = args.Query
			}
			if field == "" {
				return "", errFieldRequired
			}
			value := args.Value
			if value == "" {
				value = args.Body
			}
			if err := svc.ViewMutate(args.Ref, field, value); err != nil {
				return "", err
			}
			vr, err := svc.ViewGet(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(vr)

		case "diff":
			if args.Ref == "" {
				return "", errRefRequired
			}
			diff, err := svc.ViewDiff(args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(diff)

		case "push":
			if args.Ref == "" {
				return "", errRefRequired
			}
			issue, err := svc.ViewPush(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			if issue == nil {
				return server.JSONString(map[string]string{
					"ref":     args.Ref,
					"message": "no dirty fields to push",
				})
			}
			return server.JSONString(issue)

		case "list":
			records := svc.ViewList()
			return server.JSONString(records)

		case "dirty":
			changes := svc.ViewDirty()
			return server.JSONString(changes)

		case "push_all":
			pushed, errs := svc.ViewPushAll(ctx)
			return server.JSONString(map[string]any{
				"pushed": pushed,
				"errors": errs,
			})

		case "drop":
			if args.Ref == "" {
				return "", errRefRequired
			}
			svc.ViewDrop(args.Ref)
			return server.JSONString(map[string]string{
				"ref":     args.Ref,
				"message": "dropped from view",
			})

		case "reset":
			svc.ViewReset()
			return server.JSONString(map[string]string{
				"message": "view store reset",
			})

		default:
			return "", fmt.Errorf("%w %q", errUnknownAction, args.Action)
		}
	}
}

//nolint:gocyclo,funlen // action dispatcher
func launchHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args emceeArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		_ = int(args.Limit)

		switch args.Action {
		case "pull":
			if args.Ref == "" {
				return "", errRefRequired
			}
			ref := "reportportal:" + args.Ref
			lv, err := svc.ViewPull(ctx, ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(lv)

		case "list":
			filter := domain.LaunchFilter{
				Name:   args.Query,
				Status: args.Status,
				Limit:  int(args.Limit),
				Page:   int(args.Page),
			}
			if t, err := timeexpr.Parse(args.Since); err != nil {
				return "", fmt.Errorf("invalid since: %w", err)
			} else {
				filter.StartAfter = t
			}
			if t, err := timeexpr.Parse(args.Before); err != nil {
				return "", fmt.Errorf("invalid before: %w", err)
			} else {
				filter.StartBefore = t
			}
			launches, err := svc.ListLaunches(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(launches)

		case "get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			launch, err := svc.GetLaunch(ctx, args.Backend, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(launch)

		case "items":
			if args.Ref == "" {
				return "", errRefRequired
			}
			filter := domain.TestItemFilter{
				Name:        args.Query,
				Status:      splitCSV(args.Status),
				Limit:       int(args.Limit),
				Page:        int(args.Page),
				IncludeLogs: args.IncludeLogs,
			}
			items, err := svc.ListTestItems(ctx, args.Backend, args.Ref, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(items)

		case "search_items":
			filter := domain.TestItemFilter{
				LaunchName:  args.Query,
				Status:      splitCSV(args.Status),
				IssueType:   splitCSV(args.IssueType),
				Limit:       int(args.Limit),
				Page:        int(args.Page),
				IncludeLogs: args.IncludeLogs,
			}
			if args.CILane != "" {
				filter.LaunchAttributes = map[string]string{"ci-lane": args.CILane}
			}
			if t, err := timeexpr.Parse(args.Since); err != nil {
				return "", fmt.Errorf("invalid since: %w", err)
			} else {
				filter.Since = t
			}
			if t, err := timeexpr.Parse(args.Before); err != nil {
				return "", fmt.Errorf("invalid before: %w", err)
			} else {
				filter.Before = t
			}
			items, err := svc.SearchTestItems(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(items)

		case "item_get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			item, err := svc.GetTestItem(ctx, args.Backend, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(item)

		case "bulk_item_get":
			if args.Issues == "" {
				return "", errIssuesRequired
			}
			var ids []string
			if err := json.Unmarshal([]byte(args.Issues), &ids); err != nil {
				return "", fmt.Errorf("invalid test item IDs JSON: %w", err)
			}
			items, err := svc.GetTestItems(ctx, args.Backend, ids)
			if err != nil {
				return "", err
			}
			return server.JSONString(items)

		case "tree":
			if args.Ref == "" {
				return "", errRefRequired
			}
			tree, err := svc.LaunchItemTree(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(tree)

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
			return server.JSONString(map[string]any{"updated": len(updates)})

		// --- Dashboard operations ---

		case "dashboards":
			dashboards, err := svc.ListDashboards(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONString(dashboards)

		case "dashboard_get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			dashboard, err := svc.GetDashboard(ctx, args.Backend, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(dashboard)

		case "dashboard_create":
			if args.Title == "" {
				return "", errTitleRequired
			}
			input := domain.DashboardCreateInput{Name: args.Title, Description: args.Description}
			dashboard, err := svc.CreateDashboard(ctx, args.Backend, input)
			if err != nil {
				return "", err
			}
			return server.JSONString(dashboard)

		case "widget_add":
			if args.Ref == "" {
				return "", errRefRequired
			}
			if args.Title == "" {
				return "", errTitleRequired
			}
			input := domain.WidgetAddInput{
				Name:   args.Title,
				Type:   args.IssueType,
				Width:  int(args.Limit),
				Height: 4,
			}
			widget, err := svc.AddWidget(ctx, args.Backend, args.Ref, input)
			if err != nil {
				return "", err
			}
			return server.JSONString(widget)

		// --- Doc operations ---

		default:
			return "", fmt.Errorf("%w %q", errUnknownAction, args.Action)
		}
	}
}

//nolint:gocyclo,funlen // action dispatcher
func docHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args emceeArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		_ = int(args.Limit) // limit unused in this handler

		switch args.Action {
		case "parse":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			return server.JSONString(tree)

		case "links":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			docparse.CheckDeadLinks(tree)
			edges := docparse.ExtractLinkEdges(tree)
			return server.JSONString(map[string]any{"links": tree.Links, "edges": edges})

		case "diff":
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Body == "" {
				return "", errBodyRequired
			}
			oldTree := docparse.Parse([]byte(args.Query))
			newTree := docparse.Parse([]byte(args.Body))
			diffs := docparse.VersionDiff(oldTree, newTree)
			return server.JSONString(diffs)

		case "audit":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			return server.JSONString(map[string]any{
				"duplicates": docparse.FindDuplicateCodeBlocks(tree),
				"weights":    docparse.AnalyzeBloat(tree),
			})

		case "terms":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			terms := splitCSV(args.Body)
			var usages []docparse.TermUsage
			for _, term := range terms {
				usages = append(usages, docparse.FindTermUsage(args.Query, tree, term))
			}
			inconsistencies := docparse.FindInconsistentTerms(args.Query, terms)
			return server.JSONString(map[string]any{"usage": usages, "inconsistencies": inconsistencies})

		case "validate":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			titles := splitCSV(args.Body)
			rules := make([]docparse.TemplateRule, len(titles))
			for i, t := range titles {
				rules[i] = docparse.TemplateRule{Title: t, Required: true}
			}
			result := docparse.ValidateTemplate(tree, rules)
			return server.JSONString(result)

		case "declarations":
			if args.Query == "" {
				return "", errQueryRequired
			}
			tree := docparse.Parse([]byte(args.Query))
			decls := docparse.ExtractGoDeclarations(tree)
			return server.JSONString(decls)

		// --- PR reviews ---

		case "sync_gist":
			if args.Title == "" {
				return "", errTitleRequired
			}
			if args.Query == "" {
				return "", errQueryRequired
			}
			if args.Ref != "" {
				url, err := svc.UpdateGist(ctx, args.Backend, args.Ref, args.Title, args.Query)
				if err != nil {
					return "", err
				}
				return server.JSONString(map[string]string{"updated": args.Ref, "url": url})
			}
			id, url, err := svc.CreateGist(ctx, args.Backend, args.Title, args.Query, false)
			if err != nil {
				return "", err
			}
			return server.JSONString(map[string]string{"id": id, "url": url})

		case "sync_jira":
			if args.Ref == "" {
				return "", errRefRequired
			}
			if args.Query == "" {
				return "", errQueryRequired
			}
			desc := args.Query
			issue, err := svc.Update(ctx, args.Ref, domain.UpdateInput{Description: &desc})
			if err != nil {
				return "", err
			}
			return server.JSONString(map[string]string{"updated": issue.Ref, "url": issue.URL})

		// --- Triage ---

		default:
			return "", fmt.Errorf("%w %q", errUnknownAction, args.Action)
		}
	}
}

//nolint:gocyclo,funlen // action dispatcher
func adminHandler(svc EmceeService) server.Handler {
	return func(ctx context.Context, input json.RawMessage) (string, error) {
		var args emceeArgs
		if err := json.Unmarshal(input, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
		_ = int(args.Limit)

		switch args.Action {
		case "help":
			var sb strings.Builder
			sb.WriteString("Emcee — 6 tools. Ref format: backend:key (e.g. jira:PROJ-42).\n\n")

			sb.WriteString("issue — Issue CRUD across all backends\n")
			sb.WriteString("  list         backend [status] [assignee] [limit]          List issues. status: backlog|todo|in_progress|in_review|done|canceled.\n")
			sb.WriteString("  get          ref                                           Fetch one issue by ref.\n")
			sb.WriteString("  create       backend title [description] [priority] [assignee] [parent_id] [project_id] [issue_type] [status] [components] [fix_versions] [versions]\n")
			sb.WriteString("               priority: urgent|high|medium|low. Auto-stages on failure — use push to retry.\n")
			sb.WriteString("  update       ref [title] [description] [status] [priority] [components] [fix_versions] [resolution]\n")
			sb.WriteString("               resolution (Jira close): Done | Won't Fix | Duplicate | Cannot Reproduce\n")
			sb.WriteString("  search       backend query [limit]                         Full-text or JQL search.\n")
			sb.WriteString("  children     ref                                           Direct child issues.\n")
			sb.WriteString("  bulk_create  backend issues=JSON                           JSON array of CreateInput objects.\n")
			sb.WriteString("  bulk_update  backend issues=JSON                           JSON array of {ref, ...fields}.\n")
			sb.WriteString("  comments     ref                                           List comments on an issue.\n")
			sb.WriteString("  comment_add  ref body                                      Post a comment.\n")
			sb.WriteString("  link         ref issue_type target_ref                     Add a link (e.g. issue_type=Blocks).\n")
			sb.WriteString("  unlink       ref issue_type target_ref                     Remove a link.\n")
			sb.WriteString("  link_types   backend                                       List available link type names.\n")
			sb.WriteString("  stage        title [description] [priority] [assignee]     Draft a local issue (not yet sent to backend).\n")
			sb.WriteString("  stage_list                                                 List staged drafts.\n")
			sb.WriteString("  stage_show   stage_id                                      Inspect one staged draft.\n")
			sb.WriteString("  stage_patch  stage_id [title] [description] [priority]     Edit a staged draft.\n")
			sb.WriteString("  stage_drop   stage_id                                      Discard a staged draft.\n")
			sb.WriteString("  push         backend stage_id                              Create the staged issue in the backend.\n")
			sb.WriteString("  push_all     backend                                       Push all staged drafts to backend.\n")
			sb.WriteString("  fields       backend                                       List available field names for the backend.\n")
			sb.WriteString("  jql          backend query [limit]                         Raw JQL query (Jira only).\n")
			sb.WriteString("  prs          backend [repo] [author] [merged_after] [merged_before] [limit]  List merged pull requests.\n")
			sb.WriteString("  pr_reviews   ref                                           Reviews for a PR ref.\n")
			sb.WriteString("  pr_comments  ref                                           Comments on a PR ref.\n")
			sb.WriteString("  changelog    ref                                           Field change history for an issue.\n")
			sb.WriteString("\n")

			sb.WriteString("view — Local materialized view (Identity Map + Unit of Work)\n")
			sb.WriteString("  pull         ref                                           Fetch issue or launch into local cache.\n")
			sb.WriteString("  get          ref                                           Read cached copy.\n")
			sb.WriteString("  mutate       ref field value                               Set a field on the cached copy (not yet pushed).\n")
			sb.WriteString("  diff         ref                                           Show diff between cached and original.\n")
			sb.WriteString("  push         ref                                           Push cached mutations to the backend.\n")
			sb.WriteString("  push_all                                                   Push all dirty cached entries.\n")
			sb.WriteString("  list                                                       List all cached refs.\n")
			sb.WriteString("  dirty                                                      List refs with uncommitted mutations.\n")
			sb.WriteString("  drop         ref                                           Remove a ref from the cache.\n")
			sb.WriteString("  reset        ref                                           Discard mutations, restore to pulled state.\n")
			sb.WriteString("  Typical flow: view(pull) → view(mutate, field=status, value=done) → view(diff) → view(push)\n")
			sb.WriteString("\n")

			sb.WriteString("launch — Report Portal launches, test items, defects, dashboards\n")
			sb.WriteString("  pull         backend ref                                   Cache a launch and its items locally.\n")
			sb.WriteString("  list         backend [query] [limit] [page]                List launches. query filters by name.\n")
			sb.WriteString("  get          backend ref                                   Fetch one launch by ID.\n")
			sb.WriteString("  items        backend ref [status] [limit] [page] [include_logs]\n")
			sb.WriteString("               status: FAILED|PASSED|SKIPPED. include_logs fetches failure_message.\n")
			sb.WriteString("  search_items backend [query] [ci_lane] [status] [issue_type] [since] [before] [limit] [page] [include_logs]\n")
			sb.WriteString("               Cross-launch item search. query filters launch names (substring). ci_lane is an exact\n")
			sb.WriteString("               attribute match — use this to exclude gm/gnrd (e.g. ci_lane=telco-ft-ran-ptp).\n")
			sb.WriteString("               issue_type: ti001 (To Investigate) | pb001 (Product Bug) | ab001 (Automation Bug).\n")
			sb.WriteString("               since/before accept RFC3339, named anchors (startOfWeek, endOfDay, now…), or offsets (-7d, -2w).\n")
			sb.WriteString("  item_get     backend ref                                   Fetch one test item by ID.\n")
			sb.WriteString("  bulk_item_get backend issues=JSON                          JSON array of item IDs.\n")
			sb.WriteString("  defect_update backend issues=JSON                          JSON array of {test_item_id, issue_type, comment?}.\n")
			sb.WriteString("  dashboards   backend                                       List dashboards.\n")
			sb.WriteString("  dashboard_get backend ref                                  Fetch one dashboard.\n")
			sb.WriteString("  dashboard_create backend title [query]                     Create a dashboard.\n")
			sb.WriteString("  widget_add   backend ref title issue_type [limit]          Add a widget to a dashboard.\n")
			sb.WriteString("  Typical flow: launch(pull, ref=37337) → launch(items, status=FAILED) → launch(defect_update)\n")
			sb.WriteString("\n")

			sb.WriteString("doc — Document operations\n")
			sb.WriteString("  parse        query=<markdown>                              Parse markdown into structured sections.\n")
			sb.WriteString("  links        query=<markdown>                              Extract all hyperlinks.\n")
			sb.WriteString("  diff         query=<old_markdown> body=<new_markdown>      Diff two markdown documents.\n")
			sb.WriteString("  audit        query=<markdown>                              Audit markdown quality (broken links, etc.).\n")
			sb.WriteString("  terms        query=<markdown> body=<comma-separated terms> Check terms appear in the document.\n")
			sb.WriteString("  validate     query=<markdown> body=<required section titles> Assert required sections exist.\n")
			sb.WriteString("  declarations query=<markdown>                              Extract declarations/definitions.\n")
			sb.WriteString("  sync_gist    backend=github query=<markdown> title=<filename> [ref=<gist_id>]\n")
			sb.WriteString("               Create or update a GitHub Gist.\n")
			sb.WriteString("  sync_jira    backend=jira ref=<jira:KEY> query=<markdown>  Push markdown as Jira page body.\n")
			sb.WriteString("\n")

			sb.WriteString("admin — Meta, triage, and knowledge ledger\n")
			sb.WriteString("  help                                                       This output.\n")
			sb.WriteString("  triage       ref [limit]                                   Traverse issue graph from seed ref. limit=depth (default 3).\n")
			sb.WriteString("  triage_config                                              Show triage rate-limit and backend allow-list.\n")
			sb.WriteString("  triage_config_set [limit] [issues=JSON]                   Set triage config. issues is JSON array of backend names.\n")
			sb.WriteString("  changelog    ref                                           Field change history (alias of issue changelog).\n")
			sb.WriteString("  fields_discover backend                                    Discover and cache field mappings for backend.\n")
			sb.WriteString("  ledger_list  [backend] [status] [issue_type] [components] [limit]  List ledger entries.\n")
			sb.WriteString("  ledger_get   ref                                           Fetch one ledger entry.\n")
			sb.WriteString("  ledger_search query [limit]                                Full-text search the ledger.\n")
			sb.WriteString("  ledger_similar ref [limit]                                 Entries similar to seed ref.\n")
			sb.WriteString("  ledger_ingest backend ref title description [status] [issue_type] [components]  Add entry to ledger.\n")
			sb.WriteString("  ledger_stats                                               Ledger entry counts by type/status.\n")
			sb.WriteString("\n")

			sb.WriteString("manage — Supporting entities\n")
			sb.WriteString("  doc_list         backend [limit]                           List documents.\n")
			sb.WriteString("  doc_create       backend title content [description] [project_id]  Create a document.\n")
			sb.WriteString("  project_list     backend [limit]                           List projects.\n")
			sb.WriteString("  project_create   backend name [description]                Create a project.\n")
			sb.WriteString("  project_update   backend id [name] [description]           Update a project.\n")
			sb.WriteString("  initiative_list  backend [limit]                           List initiatives.\n")
			sb.WriteString("  initiative_create backend name [description]               Create an initiative.\n")
			sb.WriteString("  label_list       backend [limit]                           List labels.\n")
			sb.WriteString("  label_create     backend name [color]                      Create a label. color is hex.\n")
			sb.WriteString("  config_reload                                              Hot-reload emcee config from disk.\n")
			sb.WriteString("  backend_remove   backend                                   Remove a backend from the runtime config.\n")
			sb.WriteString("\n")

			sb.WriteString("Tips:\n")
			sb.WriteString("  issue(list, backend=jira) → view(pull) → view(mutate, field=status, value=done) → view(push)\n")
			sb.WriteString("  issue(search, backend=jira, query=\"bug in login\") — keyword or JQL\n")
			sb.WriteString("  launch(pull, ref=37337) → launch(items, status=FAILED, include_logs=true) → launch(defect_update)\n")
			sb.WriteString("  admin(triage, ref=jira:PROJ-42) — graph traversal for root-cause analysis\n")
			return server.JSONString(map[string]string{"help": sb.String()})

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
			return server.JSONString(graph)

		case "triage_config":
			cfg := svc.GetTriageConfig()
			return server.JSONString(cfg)

		case "triage_config_set":
			var cfg service.TriageConfig
			cfg.RateLimit = args.Limit
			if args.Issues != "" {
				if err := json.Unmarshal([]byte(args.Issues), &cfg.AllowList); err != nil {
					return "", fmt.Errorf("invalid allow_list JSON: %w", err)
				}
			}
			svc.SetTriageConfig(cfg)
			return server.JSONString(cfg)

		// --- Field discovery + JQL ---

		case "fields_discover":
			if args.Backend == "" {
				return "", errBackendRequired
			}
			mappings, err := svc.DiscoverFields(ctx, args.Backend, config.Dir())
			if err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{
				"backend":  args.Backend,
				"manifest": config.DefaultPath(args.Backend),
				"mappings": mappings,
			})

		case "ledger_list":
			filter := domain.LedgerFilter{
				Backend:   args.Backend,
				Type:      args.IssueType,
				Component: args.ComponentsStr,
				Status:    args.Status,
				Limit:     int(args.Limit),
			}
			records, err := svc.LedgerList(ctx, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(records)

		case "ledger_get":
			if args.Ref == "" {
				return "", errRefRequired
			}
			record, err := svc.LedgerGet(ctx, args.Ref)
			if err != nil {
				return "", err
			}
			return server.JSONString(record)

		case "ledger_search":
			if args.Query == "" {
				return "", errQueryRequired
			}
			searchLimit := int(args.Limit)
			if searchLimit == 0 {
				searchLimit = defaultSearchMax
			}
			records, err := svc.LedgerSearch(ctx, args.Query, searchLimit)
			if err != nil {
				return "", err
			}
			return server.JSONString(records)

		case "ledger_similar":
			if args.Ref == "" {
				return "", errRefRequired
			}
			simLimit := int(args.Limit)
			if simLimit == 0 {
				simLimit = 10
			}
			records, err := svc.LedgerSimilar(ctx, args.Ref, simLimit)
			if err != nil {
				return "", err
			}
			return server.JSONString(records)

		case "ledger_ingest":
			if args.Ref == "" {
				return "", errRefRequired
			}
			record := domain.ArtifactRecord{
				Ref:        args.Ref,
				Backend:    args.Backend,
				Type:       args.IssueType,
				Title:      args.Title,
				Status:     args.Status,
				Text:       args.Description,
				Components: splitCSV(args.ComponentsStr),
				SeenAt:     time.Now(),
				UpdatedAt:  time.Now(),
			}
			if err := svc.LedgerIngest(ctx, record); err != nil {
				return "", err
			}
			return server.JSONString(map[string]string{"ingested": args.Ref})

		case "ledger_stats":
			stats, err := svc.LedgerStats(ctx)
			if err != nil {
				return "", err
			}
			return server.JSONString(stats)

		// --- View (Local Materialized View) ---

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
			return server.JSONString(docs)

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
			return server.JSONString(doc)

		case "project_list":
			filter := domain.ProjectListFilter{Limit: limit}
			projects, err := svc.ListProjects(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(projects)

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
			return server.JSONString(proj)

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
			return server.JSONString(proj)

		case "initiative_list":
			filter := domain.InitiativeListFilter{Limit: limit}
			inits, err := svc.ListInitiatives(ctx, args.Backend, filter)
			if err != nil {
				return "", err
			}
			return server.JSONString(inits)

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
			return server.JSONString(init)

		case "label_list":
			labels, err := svc.ListLabels(ctx, args.Backend)
			if err != nil {
				return "", err
			}
			return server.JSONString(labels)

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
			return server.JSONString(label)

		case "config_reload":
			added, removed, err := svc.ReloadConfig("")
			if err != nil {
				return "", err
			}
			return server.JSONString(map[string]any{"added": added, "removed": removed})

		case "backend_remove":
			if args.Name == "" {
				return "", errNameRequired
			}
			ok := svc.RemoveBackend(args.Name)
			if !ok {
				return "", fmt.Errorf("%w: %s", errBackendNotFound, args.Name)
			}
			return server.JSONString(map[string]string{"removed": args.Name})

		default:
			return "", fmt.Errorf("%w %q (valid: doc_list, doc_create, project_list, project_create, project_update, initiative_list, initiative_create, label_list, label_create, config_reload, backend_remove)", errUnknownAction, args.Action)
		}
	}
}

func healthHandler(svc EmceeService) server.Handler {
	return func(_ context.Context, _ json.RawMessage) (string, error) {
		health := svc.Health()
		return server.JSONString(health)
	}
}

var errInvalidRef = errors.New("invalid ref (expected backend:key)")

func parseRef(ref string) (backend, key string, err error) {
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("%w: %q", errInvalidRef, ref)
	}
	return parts[0], parts[1], nil
}

// splitCSV splits a comma-separated string into a trimmed slice.
// Returns nil for empty input.
func issueWithProjectNote(issue *domain.Issue, explicitProject, configDefault string) map[string]any {
	b, _ := json.Marshal(issue)
	var m map[string]any
	_ = json.Unmarshal(b, &m)
	if explicitProject != "" {
		m["_project_source"] = "explicit"
	} else {
		m["_project_source"] = fmt.Sprintf("default from config (team: %s). Pass project_id to target a different project.", configDefault)
	}
	return m
}

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
