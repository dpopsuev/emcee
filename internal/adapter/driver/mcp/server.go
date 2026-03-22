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
	serverVersion = "0.3.0"

	defaultBackend   = "linear"
	defaultListMax   = 50
	defaultSearchMax = 20
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

const serverInstructions = `Emcee is a unified issue tracker across Linear, GitHub, and Jira. Two tools: emcee (issue CRUD: list, get, create, update, search, children, bulk_create, bulk_update) and emcee_manage (supporting entities: doc_list, doc_create, project_list, project_create, initiative_list, initiative_create, label_list, label_create). Ref format: "backend:key" (e.g. "linear:HEG-17"). Backend defaults to "linear". Status: backlog, todo, in_progress, in_review, done, canceled. Priority: urgent, high, medium, low. Bulk ops accept JSON array in issues param, auto-batch to 50.`

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
	s.AddTool(emceeTool(), emceeHandler(svc))
	s.AddTool(manageTool(), manageHandler(svc))
}

// --- Tool definitions ---

func emceeTool() gomcp.Tool {
	return gomcp.NewTool("emcee",
		gomcp.WithDescription("Issue CRUD: list, get, create, update, search, children, bulk_create, bulk_update."),
		gomcp.WithString("action", gomcp.Required(), gomcp.Description("Action: list, get, create, update, search, children, bulk_create, bulk_update")),
		gomcp.WithString("backend", gomcp.Description("Backend name (default: linear)")),
		gomcp.WithString("ref", gomcp.Description("Issue ref for get/update/children (e.g. linear:HEG-17)")),
		gomcp.WithString("title", gomcp.Description("Issue title (create)")),
		gomcp.WithString("description", gomcp.Description("Issue description (create/update)")),
		gomcp.WithString("status", gomcp.Description("Status: backlog, todo, in_progress, in_review, done, canceled")),
		gomcp.WithString("priority", gomcp.Description("Priority: urgent, high, medium, low")),
		gomcp.WithString("assignee", gomcp.Description("Assignee name (create/list filter)")),
		gomcp.WithString("parent_id", gomcp.Description("Parent issue ID for sub-issues (create)")),
		gomcp.WithString("project_id", gomcp.Description("Project ID (create)")),
		gomcp.WithString("query", gomcp.Description("Search query text (search)")),
		gomcp.WithNumber("limit", gomcp.Description("Max results (list/search)")),
		gomcp.WithString("issues", gomcp.Description("JSON array for bulk_create/bulk_update")),
	)
}

func manageTool() gomcp.Tool {
	return gomcp.NewTool("emcee_manage",
		gomcp.WithDescription("Supporting entities: doc_list, doc_create, project_list, project_create, initiative_list, initiative_create, label_list, label_create."),
		gomcp.WithString("action", gomcp.Required(), gomcp.Description("Action: doc_list, doc_create, project_list, project_create, initiative_list, initiative_create, label_list, label_create")),
		gomcp.WithString("backend", gomcp.Description("Backend name (default: linear)")),
		gomcp.WithString("title", gomcp.Description("Document title (doc_create)")),
		gomcp.WithString("name", gomcp.Description("Entity name (project/initiative/label create)")),
		gomcp.WithString("description", gomcp.Description("Description (doc/project/initiative create)")),
		gomcp.WithString("content", gomcp.Description("Markdown content (doc_create)")),
		gomcp.WithString("project_id", gomcp.Description("Link document to project (doc_create)")),
		gomcp.WithString("color", gomcp.Description("Label color hex (label_create)")),
		gomcp.WithNumber("limit", gomcp.Description("Max results (list actions)")),
	)
}

// --- Dispatchers ---

func emceeHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		action := stringArg(req, "action", "")
		backend := stringArg(req, "backend", defaultBackend)

		switch action {
		case "list":
			filter := domain.ListFilter{
				Status:   domain.Status(stringArg(req, "status", "")),
				Assignee: stringArg(req, "assignee", ""),
				Limit:    intArg(req, "limit", defaultListMax),
			}
			issues, err := svc.List(ctx, backend, filter)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issues)

		case "get":
			ref := stringArg(req, "ref", "")
			if ref == "" {
				return errResult(errors.New("ref is required")), nil
			}
			issue, err := svc.Get(ctx, ref)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issue)

		case "create":
			title := stringArg(req, "title", "")
			if title == "" {
				return errResult(errors.New("title is required")), nil
			}
			input := domain.CreateInput{
				Title:       title,
				Description: stringArg(req, "description", ""),
				Priority:    domain.ParsePriority(stringArg(req, "priority", "")),
				Assignee:    stringArg(req, "assignee", ""),
				ParentID:    stringArg(req, "parent_id", ""),
				ProjectID:   stringArg(req, "project_id", ""),
			}
			if s := stringArg(req, "status", ""); s != "" {
				input.Status = domain.Status(s)
			}
			issue, err := svc.Create(ctx, backend, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issue)

		case "update":
			ref := stringArg(req, "ref", "")
			if ref == "" {
				return errResult(errors.New("ref is required")), nil
			}
			var input domain.UpdateInput
			if v := stringArg(req, "title", ""); v != "" {
				input.Title = &v
			}
			if v := stringArg(req, "description", ""); v != "" {
				input.Description = &v
			}
			if v := stringArg(req, "status", ""); v != "" {
				s := domain.Status(v)
				input.Status = &s
			}
			if v := stringArg(req, "priority", ""); v != "" {
				p := domain.ParsePriority(v)
				input.Priority = &p
			}
			issue, err := svc.Update(ctx, ref, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issue)

		case "search":
			query := stringArg(req, "query", "")
			if query == "" {
				return errResult(errors.New("query is required")), nil
			}
			issues, err := svc.Search(ctx, backend, query, intArg(req, "limit", defaultSearchMax))
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issues)

		case "children":
			ref := stringArg(req, "ref", "")
			if ref == "" {
				return errResult(errors.New("ref is required")), nil
			}
			issues, err := svc.ListChildren(ctx, ref)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(issues)

		case "bulk_create":
			issuesJSON := stringArg(req, "issues", "")
			if issuesJSON == "" {
				return errResult(errors.New("issues is required")), nil
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

		case "bulk_update":
			issuesJSON := stringArg(req, "issues", "")
			if issuesJSON == "" {
				return errResult(errors.New("issues is required")), nil
			}
			var inputs []domain.BulkUpdateInput
			if err := json.Unmarshal([]byte(issuesJSON), &inputs); err != nil {
				return errResult(fmt.Errorf("invalid issues JSON: %w", err)), nil
			}
			result, err := svc.BulkUpdateIssues(ctx, backend, inputs)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(result)

		default:
			return errResult(fmt.Errorf("unknown action %q (valid: list, get, create, update, search, children, bulk_create, bulk_update)", action)), nil
		}
	}
}

func manageHandler(svc EmceeService) server.ToolHandlerFunc {
	return func(ctx context.Context, req gomcp.CallToolRequest) (*gomcp.CallToolResult, error) {
		action := stringArg(req, "action", "")
		backend := stringArg(req, "backend", defaultBackend)

		switch action {
		case "doc_list":
			filter := domain.DocumentListFilter{Limit: intArg(req, "limit", defaultListMax)}
			docs, err := svc.ListDocuments(ctx, backend, filter)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(docs)

		case "doc_create":
			title := stringArg(req, "title", "")
			if title == "" {
				return errResult(errors.New("title is required")), nil
			}
			input := domain.DocumentCreateInput{
				Title:     title,
				Content:   stringArg(req, "content", ""),
				ProjectID: stringArg(req, "project_id", ""),
			}
			doc, err := svc.CreateDocument(ctx, backend, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(doc)

		case "project_list":
			filter := domain.ProjectListFilter{Limit: intArg(req, "limit", defaultListMax)}
			projects, err := svc.ListProjects(ctx, backend, filter)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(projects)

		case "project_create":
			name := stringArg(req, "name", "")
			if name == "" {
				return errResult(errors.New("name is required")), nil
			}
			input := domain.ProjectCreateInput{
				Name:        name,
				Description: stringArg(req, "description", ""),
			}
			proj, err := svc.CreateProject(ctx, backend, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(proj)

		case "initiative_list":
			filter := domain.InitiativeListFilter{Limit: intArg(req, "limit", defaultListMax)}
			inits, err := svc.ListInitiatives(ctx, backend, filter)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(inits)

		case "initiative_create":
			name := stringArg(req, "name", "")
			if name == "" {
				return errResult(errors.New("name is required")), nil
			}
			input := domain.InitiativeCreateInput{
				Name:        name,
				Description: stringArg(req, "description", ""),
			}
			init, err := svc.CreateInitiative(ctx, backend, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(init)

		case "label_list":
			labels, err := svc.ListLabels(ctx, backend)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(labels)

		case "label_create":
			name := stringArg(req, "name", "")
			if name == "" {
				return errResult(errors.New("name is required")), nil
			}
			input := domain.LabelCreateInput{
				Name:  name,
				Color: stringArg(req, "color", ""),
			}
			label, err := svc.CreateLabel(ctx, backend, input)
			if err != nil {
				return errResult(err), nil
			}
			return jsonResult(label)

		default:
			return errResult(fmt.Errorf("unknown action %q (valid: doc_list, doc_create, project_list, project_create, initiative_list, initiative_create, label_list, label_create)", action)), nil
		}
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
