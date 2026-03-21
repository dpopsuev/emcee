// Package mcp implements the driver (inbound) adapter as an MCP stdio server.
package mcp

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
	gomcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	serverName    = "emcee"
	serverVersion = "0.1.0"

	toolList   = "emcee_list"
	toolGet    = "emcee_get"
	toolCreate = "emcee_create"
	toolUpdate = "emcee_update"
	toolSearch = "emcee_search"

	argBackend     = "backend"
	argRef         = "ref"
	argTitle       = "title"
	argDescription = "description"
	argStatus      = "status"
	argPriority    = "priority"
	argAssignee    = "assignee"
	argQuery       = "query"
	argLimit       = "limit"

	defaultBackend = "linear"
	defaultListMax = 50
	defaultSearchMax = 20
)

var (
	ErrRefRequired             = errors.New("ref is required")
	ErrBackendAndTitleRequired = errors.New("backend and title are required")
	ErrBackendAndQueryRequired = errors.New("backend and query are required")
)

// Serve starts the MCP server over stdio, exposing issue management tools.
func Serve(svc driver.IssueService) error {
	s := server.NewMCPServer(serverName, serverVersion)
	registerTools(s, svc)
	return server.ServeStdio(s)
}

// RegisterToolsForTesting is exported for test access.
var RegisterToolsForTesting = registerTools

func registerTools(s *server.MCPServer, svc driver.IssueService) {
	s.AddTool(listTool(), listHandler(svc))
	s.AddTool(getTool(), getHandler(svc))
	s.AddTool(createTool(), createHandler(svc))
	s.AddTool(updateTool(), updateHandler(svc))
	s.AddTool(searchTool(), searchHandler(svc))
}

// --- Tool definitions ---

func listTool() gomcp.Tool {
	return gomcp.NewTool(toolList,
		gomcp.WithDescription("List issues from a backend. Returns issues matching the given filters."),
		gomcp.WithString(argBackend, gomcp.Description("Backend name: linear, github, jira"), gomcp.DefaultString(defaultBackend)),
		gomcp.WithString(argStatus, gomcp.Description("Filter by status: backlog, todo, in_progress, in_review, done, canceled")),
		gomcp.WithString(argAssignee, gomcp.Description("Filter by assignee name")),
		gomcp.WithNumber(argLimit, gomcp.Description("Max results to return")),
	)
}

func getTool() gomcp.Tool {
	return gomcp.NewTool(toolGet,
		gomcp.WithDescription("Get a single issue by canonical ref (e.g. linear:HEG-17, github:owner/repo#42)."),
		gomcp.WithString(argRef, gomcp.Required(), gomcp.Description("Canonical issue reference (backend:key)")),
	)
}

func createTool() gomcp.Tool {
	return gomcp.NewTool(toolCreate,
		gomcp.WithDescription("Create a new issue on a backend."),
		gomcp.WithString(argBackend, gomcp.Required(), gomcp.Description("Backend name: linear, github, jira")),
		gomcp.WithString(argTitle, gomcp.Required(), gomcp.Description("Issue title")),
		gomcp.WithString(argDescription, gomcp.Description("Issue description")),
		gomcp.WithString(argPriority, gomcp.Description("Priority: urgent, high, medium, low")),
		gomcp.WithString(argStatus, gomcp.Description("Initial status")),
	)
}

func updateTool() gomcp.Tool {
	return gomcp.NewTool(toolUpdate,
		gomcp.WithDescription("Update an existing issue by canonical ref."),
		gomcp.WithString(argRef, gomcp.Required(), gomcp.Description("Canonical issue reference (backend:key)")),
		gomcp.WithString(argTitle, gomcp.Description("New title")),
		gomcp.WithString(argDescription, gomcp.Description("New description")),
		gomcp.WithString(argStatus, gomcp.Description("New status")),
		gomcp.WithString(argPriority, gomcp.Description("New priority")),
	)
}

func searchTool() gomcp.Tool {
	return gomcp.NewTool(toolSearch,
		gomcp.WithDescription("Search issues by text query across a backend."),
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
