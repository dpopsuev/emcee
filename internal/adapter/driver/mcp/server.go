// Package mcp implements the driver (inbound) adapter as an MCP stdio server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "emcee"
	serverVersion = "0.2.0"

	toolList   = "emcee_list"
	toolGet    = "emcee_get"
	toolCreate = "emcee_create"
	toolUpdate = "emcee_update"
	toolSearch = "emcee_search"

	toolDocList       = "emcee_doc_list"
	toolDocCreate     = "emcee_doc_create"
	toolProjectList   = "emcee_project_list"
	toolProjectCreate = "emcee_project_create"
	toolInitList      = "emcee_initiative_list"
	toolInitCreate    = "emcee_initiative_create"
	toolLabelList     = "emcee_label_list"
	toolLabelCreate   = "emcee_label_create"
	toolBulkCreate    = "emcee_bulk_create"

	argBackend     = "backend"
	argRef         = "ref"
	argTitle       = "title"
	argName        = "name"
	argDescription = "description"
	argContent     = "content"
	argProjectID   = "project_id"
	argStatus      = "status"
	argPriority    = "priority"
	argAssignee    = "assignee"
	argQuery       = "query"
	argLimit       = "limit"
	argIssues      = "issues"
	argColor       = "color"

	defaultBackend   = "linear"
	defaultListMax   = 50
	defaultSearchMax = 20
)

var (
	ErrRefRequired             = errors.New("ref is required")
	ErrBackendAndTitleRequired = errors.New("backend and title are required")
	ErrBackendAndQueryRequired = errors.New("backend and query are required")
	ErrBackendAndNameRequired  = errors.New("backend and name are required")
)

// EmceeService combines all driver port interfaces.
type EmceeService interface {
	driver.IssueService
	driver.DocumentService
	driver.ProjectService
	driver.InitiativeService
	driver.LabelService
	driver.BulkService
}

const serverInstructions = `Emcee is a unified issue tracker across Linear, GitHub, and Jira. Ref format: "backend:key" (e.g. "linear:HEG-17"). Backend defaults to "linear" for list ops, required for create/update. Entities: Issues (work), Documents (rich text, no status), Projects (group issues), Initiatives (group projects), Labels (tags). Status: backlog, todo, in_progress, in_review, done, canceled. Priority: urgent, high, medium, low. Use bulk_create for batch ops (auto-batches to 50).`

// Serve starts the MCP server over stdio, exposing issue management tools.
func Serve(svc EmceeService) error {
	s := server.NewMCPServer(serverName, serverVersion,
		server.WithInstructions(serverInstructions),
	)
	registerTools(s, svc)
	return server.ServeStdio(s)
}

// RegisterToolsForTesting is exported for test access.
var RegisterToolsForTesting = registerTools

func registerTools(s *server.MCPServer, svc EmceeService) {
	s.AddTool(listTool(), listHandler(svc))
	s.AddTool(getTool(), getHandler(svc))
	s.AddTool(createTool(), createHandler(svc))
	s.AddTool(updateTool(), updateHandler(svc))
	s.AddTool(searchTool(), searchHandler(svc))
	s.AddTool(docListTool(), docListHandler(svc))
	s.AddTool(docCreateTool(), docCreateHandler(svc))
	s.AddTool(projectListTool(), projectListHandler(svc))
	s.AddTool(projectCreateTool(), projectCreateHandler(svc))
	s.AddTool(initListTool(), initListHandler(svc))
	s.AddTool(initCreateTool(), initCreateHandler(svc))
	s.AddTool(labelListTool(), labelListHandler(svc))
	s.AddTool(labelCreateTool(), labelCreateHandler(svc))
	s.AddTool(bulkCreateTool(), bulkCreateHandler(svc))
}

// --- Tool definitions ---

func listTool() gomcp.Tool {
	return gomcp.NewTool(toolList,
		gomcp.WithDescription("List issues. Filter by status and assignee."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name: linear, github, jira"), gomcp.DefaultString(defaultBackend)),
		gomcp.WithString(argStatus, gomcp.Description("Filter by status: backlog, todo, in_progress, in_review, done, canceled")),
		gomcp.WithString(argAssignee, gomcp.Description("Filter by assignee name")),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

func getTool() gomcp.Tool {
	return gomcp.NewTool(toolGet,
		gomcp.WithDescription("Get issue details by ref (e.g. linear:HEG-17)."),
		gomcp.WithString(argRef, gomcp.Required(), gomcp.Description("Canonical issue reference (backend:key)")),
	)
}

func createTool() gomcp.Tool {
	return gomcp.NewTool(toolCreate,
		gomcp.WithDescription("Create an issue. Supports parent_id (sub-issues), project_id, and assignee (name, auto-resolved)."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name: linear, github, jira")),
		gomcp.WithString(argTitle, gomcp.Required(), gomcp.Description("Issue title")),
		gomcp.WithString(argDescription, gomcp.Description("Issue description")),
		gomcp.WithString(argPriority, gomcp.Description("Priority: urgent, high, medium, low")),
		gomcp.WithString(argStatus, gomcp.Description("Initial status")),
	)
}

func updateTool() gomcp.Tool {
	return gomcp.NewTool(toolUpdate,
		gomcp.WithDescription("Update an issue by ref. Only provided fields are changed."),
		gomcp.WithString(argRef, gomcp.Required(), gomcp.Description("Canonical issue reference (backend:key)")),
		gomcp.WithString(argTitle, gomcp.Description("New title")),
		gomcp.WithString(argDescription, gomcp.Description("New description")),
		gomcp.WithString(argStatus, gomcp.Description("New status")),
		gomcp.WithString(argPriority, gomcp.Description("New priority")),
	)
}

func searchTool() gomcp.Tool {
	return gomcp.NewTool(toolSearch,
		gomcp.WithDescription("Full-text search across issues."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name: linear, github, jira")),
		gomcp.WithString(argQuery, gomcp.Required(), gomcp.Description("Search query text")),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

// --- Handlers ---

func listHandler(svc driver.IssueService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, defaultBackend)
		filter := domain.ListFilter{
			Status:   domain.Status(stringArg(req, argStatus, "")),
			Assignee: stringArg(req, argAssignee, ""),
			Limit:    intArg(req, argLimit, defaultListMax),
		}

		issues, err := svc.List(ctx, backend, filter)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(issues)
	}
}

func getHandler(svc driver.IssueService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		ref := stringArg(req, argRef, "")
		if ref == "" {
			return errResult(ErrRefRequired), nil
		}

		issue, err := svc.Get(ctx, ref)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(issue)
	}
}

func createHandler(svc driver.IssueService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		title := stringArg(req, argTitle, "")
		if backend == "" || title == "" {
			return errResult(ErrBackendAndTitleRequired), nil
		}

		input := domain.CreateInput{
			Title:       title,
			Description: stringArg(req, argDescription, ""),
			Priority:    domain.ParsePriority(stringArg(req, argPriority, "")),
		}
		if s := stringArg(req, argStatus, ""); s != "" {
			input.Status = domain.Status(s)
		}

		issue, err := svc.Create(ctx, backend, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(issue)
	}
}

func updateHandler(svc driver.IssueService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		ref := stringArg(req, argRef, "")
		if ref == "" {
			return errResult(ErrRefRequired), nil
		}

		var input domain.UpdateInput
		if v := stringArg(req, argTitle, ""); v != "" {
			input.Title = &v
		}
		if v := stringArg(req, argDescription, ""); v != "" {
			input.Description = &v
		}
		if v := stringArg(req, argStatus, ""); v != "" {
			s := domain.Status(v)
			input.Status = &s
		}
		if v := stringArg(req, argPriority, ""); v != "" {
			p := domain.ParsePriority(v)
			input.Priority = &p
		}

		issue, err := svc.Update(ctx, ref, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(issue)
	}
}

func searchHandler(svc driver.IssueService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		query := stringArg(req, argQuery, "")
		if backend == "" || query == "" {
			return errResult(ErrBackendAndQueryRequired), nil
		}

		issues, err := svc.Search(ctx, backend, query, intArg(req, argLimit, defaultSearchMax))
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(issues)
	}
}

// --- Document tools ---

func docListTool() gomcp.Tool {
	return gomcp.NewTool(toolDocList,
		gomcp.WithDescription("List documents (rich text, no status/priority)."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name"), gomcp.DefaultString(defaultBackend)),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

func docListHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, defaultBackend)
		filter := domain.DocumentListFilter{Limit: intArg(req, argLimit, defaultListMax)}
		docs, err := svc.ListDocuments(ctx, backend, filter)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(docs)
	}
}

func docCreateTool() gomcp.Tool {
	return gomcp.NewTool(toolDocCreate,
		gomcp.WithDescription("Create a document. Content is markdown. Optionally link to a project via project_id."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name")),
		gomcp.WithString(argTitle, gomcp.Required(), gomcp.Description("Document title")),
		gomcp.WithString(argContent, gomcp.Description("Document content (markdown)")),
		gomcp.WithString(argProjectID, gomcp.Description("Project ID to link the document to")),
	)
}

func docCreateHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		title := stringArg(req, argTitle, "")
		if backend == "" || title == "" {
			return errResult(ErrBackendAndTitleRequired), nil
		}
		input := domain.DocumentCreateInput{
			Title:     title,
			Content:   stringArg(req, argContent, ""),
			ProjectID: stringArg(req, argProjectID, ""),
		}
		doc, err := svc.CreateDocument(ctx, backend, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(doc)
	}
}

// --- Project tools ---

func projectListTool() gomcp.Tool {
	return gomcp.NewTool(toolProjectList,
		gomcp.WithDescription("List projects (group issues and documents)."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name"), gomcp.DefaultString(defaultBackend)),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

func projectListHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, defaultBackend)
		filter := domain.ProjectListFilter{Limit: intArg(req, argLimit, defaultListMax)}
		projects, err := svc.ListProjects(ctx, backend, filter)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(projects)
	}
}

func projectCreateTool() gomcp.Tool {
	return gomcp.NewTool(toolProjectCreate,
		gomcp.WithDescription("Create a project. Use returned ID as project_id in issues/documents."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name")),
		gomcp.WithString(argName, gomcp.Required(), gomcp.Description("Project name")),
		gomcp.WithString(argDescription, gomcp.Description("Project description")),
	)
}

func projectCreateHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		name := stringArg(req, argName, "")
		if backend == "" || name == "" {
			return errResult(ErrBackendAndNameRequired), nil
		}
		input := domain.ProjectCreateInput{
			Name:        name,
			Description: stringArg(req, argDescription, ""),
		}
		proj, err := svc.CreateProject(ctx, backend, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(proj)
	}
}

// --- Initiative tools ---

func initListTool() gomcp.Tool {
	return gomcp.NewTool(toolInitList,
		gomcp.WithDescription("List initiatives (strategic objectives, group projects)."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name"), gomcp.DefaultString(defaultBackend)),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

func initListHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, defaultBackend)
		filter := domain.InitiativeListFilter{Limit: intArg(req, argLimit, defaultListMax)}
		inits, err := svc.ListInitiatives(ctx, backend, filter)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(inits)
	}
}

func initCreateTool() gomcp.Tool {
	return gomcp.NewTool(toolInitCreate,
		gomcp.WithDescription("Create an initiative (strategic objective grouping projects)."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name")),
		gomcp.WithString(argName, gomcp.Required(), gomcp.Description("Initiative name")),
		gomcp.WithString(argDescription, gomcp.Description("Initiative description")),
	)
}

func initCreateHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		name := stringArg(req, argName, "")
		if backend == "" || name == "" {
			return errResult(ErrBackendAndNameRequired), nil
		}
		input := domain.InitiativeCreateInput{
			Name:        name,
			Description: stringArg(req, argDescription, ""),
		}
		init, err := svc.CreateInitiative(ctx, backend, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(init)
	}
}

// --- Label tools ---

func labelListTool() gomcp.Tool {
	return gomcp.NewTool(toolLabelList,
		gomcp.WithDescription("List labels (team-scoped issue tags)."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name"), gomcp.DefaultString(defaultBackend)),
	)
}

func labelListHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, defaultBackend)
		labels, err := svc.ListLabels(ctx, backend)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(labels)
	}
}

func labelCreateTool() gomcp.Tool {
	return gomcp.NewTool(toolLabelCreate,
		gomcp.WithDescription("Create a label (team-scoped issue tag)."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name")),
		gomcp.WithString(argName, gomcp.Required(), gomcp.Description("Label name")),
		gomcp.WithString(argColor, gomcp.Description("Label color (hex)")),
	)
}

func labelCreateHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		name := stringArg(req, argName, "")
		if backend == "" || name == "" {
			return errResult(ErrBackendAndNameRequired), nil
		}
		input := domain.LabelCreateInput{
			Name:  name,
			Color: stringArg(req, argColor, ""),
		}
		label, err := svc.CreateLabel(ctx, backend, input)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(label)
	}
}

// --- Bulk tools ---

func bulkCreateTool() gomcp.Tool {
	return gomcp.NewTool(toolBulkCreate,
		gomcp.WithDescription("Bulk-create issues from JSON array. Auto-batches to 50 per call. Fields: title (required), description, priority, parent_id, project_id."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name")),
		gomcp.WithString(argIssues, gomcp.Required(), gomcp.Description("JSON array of issue objects with title, description, priority, parent_id, project_id fields")),
	)
}

func bulkCreateHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		backend := stringArg(req, argBackend, "")
		issuesJSON := stringArg(req, argIssues, "")
		if backend == "" || issuesJSON == "" {
			return errResult(errors.New("backend and issues are required")), nil
		}

		var inputs []domain.CreateInput
		if err := json.Unmarshal([]byte(issuesJSON), &inputs); err != nil {
			return errResult(fmt.Errorf("invalid issues JSON: %w", err)), nil
		}

		result, err := svc.BulkCreateIssues(ctx, backend, inputs)
		if err != nil {
			return errResult(err), nil
		}
		return jsonResult(result)
	}
}

// --- Helpers ---

func stringArg(req gomcp.CallToolRequest, name, fallback string) string {
	if v, ok := req.GetArguments()[name].(string); ok && v != "" {
		return v
	}
	return fallback
}

func intArg(req gomcp.CallToolRequest, name string, fallback int) int {
	if v, ok := req.GetArguments()[name].(float64); ok {
		return int(v)
	}
	return fallback
}

func jsonResult(v any) (*gomcp.CallToolResult, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errResult(err), nil
	}
	return &gomcp.CallToolResult{
		Content: []gomcp.Content{gomcp.NewTextContent(string(data))},
	}, nil
}

func errResult(err error) *gomcp.CallToolResult {
	return &gomcp.CallToolResult{
		Content: []gomcp.Content{gomcp.NewTextContent(err.Error())},
		IsError: true,
	}
}
