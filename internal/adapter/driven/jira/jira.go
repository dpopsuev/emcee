// Package jira implements the driven (outbound) adapter for Jira's REST API v2.
package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

const (
	BackendName = "jira"

	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
)

var (
	ErrIssueNotFound       = errors.New("issue not found")
	ErrCreateFailed        = errors.New("issue creation failed")
	ErrProjectEmpty        = errors.New("project key is required")
	ErrAPIError            = errors.New("jira API error")
	ErrProjectCreate       = errors.New("project creation not supported via Jira REST API v2")
	ErrProjectUpdate       = errors.New("project update not supported via Jira REST API v2")
	ErrLabelCreateImplicit = errors.New("labels in Jira are created implicitly by adding them to issues")
	ErrNoTransition        = errors.New("no transition found matching status")
)

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository   = (*Repository)(nil)
	_ driven.ProjectRepository = (*Repository)(nil)
	_ driven.LabelRepository   = (*Repository)(nil)
	_ driven.CommentRepository = (*Repository)(nil)
	_ driven.FieldRepository   = (*Repository)(nil)
	_ driven.JQLRepository     = (*Repository)(nil)
)

// Repository implements driven.IssueRepository for Jira.
type Repository struct {
	baseURL string
	email   string
	token   string
	project string
	client  *http.Client
}

// New creates a Jira repository.
func New(baseURL, email, token, project string) (*Repository, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Repository{
		baseURL: baseURL,
		email:   email,
		token:   token,
		project: project,
		client:  &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (r *Repository) Name() string { return BackendName }

// api makes an authenticated request to the Jira REST API.
func (r *Repository) api(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, bodyReader)
	if err != nil {
		return err
	}
	req.SetBasicAuth(r.email, r.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == http.StatusNotFound {
		return ErrIssueNotFound
	}
	if resp.StatusCode == http.StatusNoContent {
		return nil
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		limit, remaining, reset := adapterdriven.ParseRateLimitHeaders(resp.Header)
		return &adapterdriven.RateLimitError{
			Backend:    BackendName,
			RetryAfter: adapterdriven.ParseRetryAfter(resp.Header.Get("Retry-After")),
			Limit:      limit,
			Remaining:  remaining,
			Reset:      reset,
			Message:    string(respBody),
		}
	}
	if resp.StatusCode >= 400 {
		errMsg := parseJiraError(respBody)
		adapterdriven.LogAPIError(ctx, BackendName, method, path, resp.StatusCode, errMsg)
		return fmt.Errorf("%w: %d %s", ErrAPIError, resp.StatusCode, errMsg)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// parseJiraError extracts human-readable error messages from Jira's error response.
// Jira returns: {"errorMessages":["msg1"],"errors":{"field":"reason"}}
func parseJiraError(body []byte) string {
	var jiraErr struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	if err := json.Unmarshal(body, &jiraErr); err != nil {
		return adapterdriven.SanitizeError(string(body))
	}

	parts := make([]string, 0, len(jiraErr.ErrorMessages)+len(jiraErr.Errors))
	parts = append(parts, jiraErr.ErrorMessages...)
	for field, msg := range jiraErr.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	if len(parts) == 0 {
		return adapterdriven.SanitizeError(string(body))
	}
	return strings.Join(parts, "; ")
}

// --- Issue operations ---

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "list", slog.String(adapterdriven.LogKeyProject, r.project))
	project := r.project
	if filter.Project != "" {
		project = filter.Project
	}

	clauses := make([]string, 0, 4)
	if project != "" {
		clauses = append(clauses, fmt.Sprintf("project = %q", project))
	}
	if filter.Status != "" {
		clauses = append(clauses, fmt.Sprintf("status = %q", mapStatusToJira(filter.Status)))
	}
	if filter.Assignee != "" {
		clauses = append(clauses, fmt.Sprintf("assignee = %q", filter.Assignee))
	}
	for _, l := range filter.Labels {
		clauses = append(clauses, fmt.Sprintf("labels = %q", l))
	}

	jql := strings.Join(clauses, " AND ")
	if jql == "" {
		jql = "ORDER BY created DESC"
	} else {
		jql += " ORDER BY created DESC"
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	return r.searchJQL(ctx, jql, limit)
}

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "get", slog.String(adapterdriven.LogKeyIssueKey, key))
	var raw jiraIssue
	path := fmt.Sprintf("/rest/api/2/issue/%s?fields=summary,status,priority,assignee,description,labels,created,updated,project,issuetype,resolution,fixVersions,components", key)
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	issue := raw.toDomain()
	return &issue, nil
}

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "create", slog.String(adapterdriven.LogKeyProject, r.project), slog.String(adapterdriven.LogKeyTitle, input.Title))
	project := r.project
	if input.ProjectID != "" {
		project = input.ProjectID
	}
	if project == "" {
		return nil, ErrProjectEmpty
	}

	issueType := input.IssueType
	if issueType == "" {
		issueType = "Task"
	}
	body := map[string]any{
		"fields": map[string]any{
			"project":     map[string]string{"key": project},
			"summary":     input.Title,
			"description": input.Description,
			"issuetype":   map[string]string{"name": issueType},
			"labels":      input.Labels,
		},
	}

	if input.Priority != domain.PriorityNone {
		body["fields"].(map[string]any)["priority"] = map[string]string{
			"name": mapPriorityToJira(input.Priority),
		}
	}
	if len(input.Versions) > 0 {
		versions := make([]map[string]string, len(input.Versions))
		for i, v := range input.Versions {
			versions[i] = map[string]string{"name": v}
		}
		body["fields"].(map[string]any)["versions"] = versions
	}
	if len(input.FixVersions) > 0 {
		fv := make([]map[string]string, len(input.FixVersions))
		for i, v := range input.FixVersions {
			fv[i] = map[string]string{"name": v}
		}
		body["fields"].(map[string]any)["fixVersions"] = fv
	}
	if len(input.Components) > 0 {
		comps := make([]map[string]string, len(input.Components))
		for i, c := range input.Components {
			comps[i] = map[string]string{"name": c}
		}
		body["fields"].(map[string]any)["components"] = comps
	}

	var result struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}
	if err := r.api(ctx, "POST", "/rest/api/2/issue", body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateFailed, err)
	}

	return r.Get(ctx, result.Key)
}

func (r *Repository) Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "update", slog.String(adapterdriven.LogKeyIssueKey, key))
	fields := map[string]any{}

	if input.Title != nil {
		fields["summary"] = *input.Title
	}
	if input.Description != nil {
		fields["description"] = *input.Description
	}
	if input.Priority != nil {
		fields["priority"] = map[string]string{
			"name": mapPriorityToJira(*input.Priority),
		}
	}
	if input.Labels != nil {
		fields["labels"] = input.Labels
	}
	if len(input.Components) > 0 {
		comps := make([]map[string]string, len(input.Components))
		for i, c := range input.Components {
			comps[i] = map[string]string{"name": c}
		}
		fields["components"] = comps
	}
	if len(input.FixVersions) > 0 {
		fv := make([]map[string]string, len(input.FixVersions))
		for i, v := range input.FixVersions {
			fv[i] = map[string]string{"name": v}
		}
		fields["fixVersions"] = fv
	}

	if len(fields) > 0 {
		body := map[string]any{"fields": fields}
		if err := r.api(ctx, "PUT", "/rest/api/2/issue/"+key, body, nil); err != nil {
			return nil, err
		}
	}

	if input.Status != nil {
		var resolution string
		if input.Resolution != nil {
			resolution = *input.Resolution
		}
		if err := r.transitionTo(ctx, key, *input.Status, resolution); err != nil {
			return nil, fmt.Errorf("transition: %w", err)
		}
	}

	return r.Get(ctx, key)
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "search", slog.String(adapterdriven.LogKeyQuery, query))
	if limit <= 0 {
		limit = defaultLimit
	}
	// Search across all accessible projects, or scoped to default project if configured
	var jql string
	if r.project != "" {
		jql = fmt.Sprintf("project = %q AND text ~ %q ORDER BY created DESC", r.project, query)
	} else {
		jql = fmt.Sprintf("text ~ %q ORDER BY created DESC", query)
	}
	return r.searchJQL(ctx, jql, limit)
}

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_children", slog.String(adapterdriven.LogKeyIssueKey, key))
	jql := fmt.Sprintf("parent = %s ORDER BY created ASC", key)
	return r.searchJQL(ctx, jql, defaultLimit)
}

// --- Project operations ---

func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_projects")
	var result []jiraProject
	if err := r.api(ctx, "GET", "/rest/api/2/project?recent=20", nil, &result); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	projects := make([]domain.Project, 0, len(result))
	for i, p := range result {
		if i >= limit {
			break
		}
		projects = append(projects, domain.Project{
			ID:   p.Key,
			Name: p.Name,
			URL:  r.baseURL + "/browse/" + p.Key,
		})
	}
	return projects, nil
}

func (r *Repository) CreateProject(_ context.Context, _ domain.ProjectCreateInput) (*domain.Project, error) {
	return nil, ErrProjectCreate
}

func (r *Repository) UpdateProject(_ context.Context, _ string, _ domain.ProjectUpdateInput) (*domain.Project, error) {
	return nil, ErrProjectUpdate
}

// --- Label operations ---

func (r *Repository) ListLabels(ctx context.Context) ([]domain.Label, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_labels")
	var raw []string
	if err := r.api(ctx, "GET", "/rest/api/2/label", nil, &raw); err != nil {
		return nil, err
	}
	labels := make([]domain.Label, 0, len(raw))
	for _, name := range raw {
		labels = append(labels, domain.Label{
			ID:   name,
			Name: name,
		})
	}
	return labels, nil
}

func (r *Repository) CreateLabel(_ context.Context, _ domain.LabelCreateInput) (*domain.Label, error) {
	return nil, ErrLabelCreateImplicit
}

// --- Internal helpers ---

func (r *Repository) searchJQL(ctx context.Context, jql string, limit int) ([]domain.Issue, error) {
	path := fmt.Sprintf("/rest/api/3/search/jql?jql=%s&maxResults=%d&fields=summary,status,priority,assignee,description,labels,created,updated,project,issuetype,resolution,fixVersions,components",
		url.QueryEscape(jql), limit)

	var result struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(result.Issues))
	for i := range result.Issues {
		issues = append(issues, result.Issues[i].toDomain())
	}
	return issues, nil
}

func (r *Repository) transitionTo(ctx context.Context, key string, status domain.Status, resolution string) error {
	var result struct {
		Transitions []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"transitions"`
	}
	if err := r.api(ctx, "GET", "/rest/api/2/issue/"+key+"/transitions", nil, &result); err != nil {
		return err
	}

	target := mapStatusToJira(status)
	for _, t := range result.Transitions {
		if strings.EqualFold(t.Name, target) {
			body := map[string]any{
				"transition": map[string]string{"id": t.ID},
			}
			if resolution != "" {
				body["fields"] = map[string]any{
					"resolution": map[string]string{"name": resolution},
				}
			}
			return r.api(ctx, "POST", "/rest/api/2/issue/"+key+"/transitions", body, nil)
		}
	}

	return fmt.Errorf("%w %q (available: %v)", ErrNoTransition, target, transitionNames(result.Transitions))
}

func transitionNames(transitions []struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}) []string {
	names := make([]string, len(transitions))
	for i, t := range transitions {
		names[i] = t.Name
	}
	return names
}

// --- Jira API types ---

type jiraIssue struct {
	ID     string `json:"id"`
	Key    string `json:"key"`
	Self   string `json:"self"`
	Fields struct {
		Summary     string          `json:"summary"`
		Description json.RawMessage `json:"description"`
		Status      struct {
			Name           string `json:"name"`
			StatusCategory struct {
				Key string `json:"key"`
			} `json:"statusCategory"`
		} `json:"status"`
		Priority *struct {
			Name string `json:"name"`
		} `json:"priority"`
		Assignee *struct {
			DisplayName string `json:"displayName"`
		} `json:"assignee"`
		Labels  []string `json:"labels"`
		Project struct {
			Key  string `json:"key"`
			Name string `json:"name"`
		} `json:"project"`
		IssueType *struct {
			Name string `json:"name"`
		} `json:"issuetype"`
		Resolution *struct {
			Name string `json:"name"`
		} `json:"resolution"`
		FixVersions []struct {
			Name string `json:"name"`
		} `json:"fixVersions"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
		Created string `json:"created"`
		Updated string `json:"updated"`
	} `json:"fields"`
}

type jiraProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

func (j *jiraIssue) toDomain() domain.Issue {
	issue := domain.Issue{
		Ref:    BackendName + ":" + j.Key,
		ID:     j.ID,
		Key:    j.Key,
		Title:  j.Fields.Summary,
		Status: mapStatusFromJira(j.Fields.Status.StatusCategory.Key),
		Labels: j.Fields.Labels,
	}

	issue.Description = extractDescription(j.Fields.Description)
	if j.Fields.Priority != nil {
		issue.Priority = mapPriorityFromJira(j.Fields.Priority.Name)
	}
	if j.Fields.Assignee != nil {
		issue.Assignee = j.Fields.Assignee.DisplayName
	}
	if j.Fields.Project.Key != "" {
		issue.Project = j.Fields.Project.Key
	}
	if j.Fields.IssueType != nil {
		issue.IssueType = j.Fields.IssueType.Name
	}
	if j.Fields.Resolution != nil {
		issue.Resolution = j.Fields.Resolution.Name
	}
	for _, fv := range j.Fields.FixVersions {
		issue.FixVersions = append(issue.FixVersions, fv.Name)
	}
	for _, c := range j.Fields.Components {
		issue.Components = append(issue.Components, c.Name)
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000+0000", j.Fields.Created); err == nil {
		issue.CreatedAt = t
	}
	if t, err := time.Parse("2006-01-02T15:04:05.000+0000", j.Fields.Updated); err == nil {
		issue.UpdatedAt = t
	}

	// Build URL from self link: strip /rest/api/... and append /browse/KEY
	if j.Self != "" {
		if idx := strings.Index(j.Self, "/rest/"); idx > 0 {
			issue.URL = j.Self[:idx] + "/browse/" + j.Key
		}
	}

	return issue
}

// extractDescription handles both v2 (plain string) and v3 (ADF object) description formats.
func extractDescription(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	// Try plain string first (v2 API)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	// ADF object (v3 API) — extract text content recursively
	var doc adfNode
	if err := json.Unmarshal(raw, &doc); err == nil {
		var b strings.Builder
		extractADFText(&doc, &b)
		return strings.TrimSpace(b.String())
	}
	return string(raw)
}

type adfNode struct {
	Type    string    `json:"type"`
	Text    string    `json:"text,omitempty"`
	Content []adfNode `json:"content,omitempty"`
}

func extractADFText(node *adfNode, b *strings.Builder) {
	if node.Text != "" {
		b.WriteString(node.Text)
	}
	for i := range node.Content {
		extractADFText(&node.Content[i], b)
	}
	if node.Type == "paragraph" || node.Type == "heading" {
		b.WriteString("\n")
	}
}

// --- Status mapping ---
// Jira statusCategory keys: "new", "indeterminate", "done", "undefined"

func mapStatusFromJira(categoryKey string) domain.Status {
	switch categoryKey {
	case "new":
		return domain.StatusTodo
	case "indeterminate":
		return domain.StatusInProgress
	case "done":
		return domain.StatusDone
	default:
		return domain.StatusBacklog
	}
}

func mapStatusToJira(status domain.Status) string {
	switch status {
	case domain.StatusBacklog:
		return "Backlog"
	case domain.StatusTodo:
		return "New"
	case domain.StatusInProgress:
		return "IN_PROGRESS"
	case domain.StatusInReview:
		return "ON_QA"
	case domain.StatusDone:
		return "Verified"
	case domain.StatusCanceled:
		return "Closed"
	default:
		return "New"
	}
}

// --- Priority mapping ---

func mapPriorityFromJira(name string) domain.Priority {
	switch strings.ToLower(name) {
	case "blocker", "critical":
		return domain.PriorityUrgent
	case "major":
		return domain.PriorityHigh
	case "normal", "minor":
		return domain.PriorityMedium
	case "trivial":
		return domain.PriorityLow
	default:
		return domain.PriorityNone
	}
}

func mapPriorityToJira(p domain.Priority) string {
	switch p {
	case domain.PriorityUrgent:
		return "Critical"
	case domain.PriorityHigh:
		return "Major"
	case domain.PriorityMedium:
		return "Normal"
	case domain.PriorityLow:
		return "Minor"
	default:
		return "Normal"
	}
}

// --- Comment operations ---

type jiraComment struct {
	ID      string `json:"id"`
	Body    string `json:"body"`
	Created string `json:"created"`
	Updated string `json:"updated"`
	Author  struct {
		DisplayName string `json:"displayName"`
	} `json:"author"`
}

func (jc jiraComment) toDomain() domain.Comment {
	c := domain.Comment{
		ID:     jc.ID,
		Body:   jc.Body,
		Author: jc.Author.DisplayName,
	}
	c.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000-0700", jc.Created)
	c.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000-0700", jc.Updated)
	return c
}

func (r *Repository) ListComments(ctx context.Context, key string) ([]domain.Comment, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_comments", slog.String(adapterdriven.LogKeyIssueKey, key))
	path := fmt.Sprintf("/rest/api/2/issue/%s/comment", key)
	var result struct {
		Comments []jiraComment `json:"comments"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	comments := make([]domain.Comment, len(result.Comments))
	for i := range result.Comments {
		comments[i] = result.Comments[i].toDomain()
	}
	return comments, nil
}

func (r *Repository) AddComment(ctx context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error) {
	adapterdriven.LogWrite(ctx, BackendName, "add_comment", slog.String(adapterdriven.LogKeyIssueKey, key))
	path := fmt.Sprintf("/rest/api/2/issue/%s/comment", key)
	body := map[string]string{"body": input.Body}
	var raw jiraComment
	if err := r.api(ctx, "POST", path, body, &raw); err != nil {
		return nil, err
	}
	c := raw.toDomain()
	return &c, nil
}

// --- Field discovery ---

func (r *Repository) ListFields(ctx context.Context) ([]domain.Field, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_fields")
	var raw []struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Custom bool   `json:"custom"`
		Schema *struct {
			Type string `json:"type"`
		} `json:"schema"`
	}
	if err := r.api(ctx, "GET", "/rest/api/2/field", nil, &raw); err != nil {
		return nil, err
	}
	fields := make([]domain.Field, 0, len(raw))
	for i := range raw {
		f := domain.Field{
			ID:     raw[i].ID,
			Name:   raw[i].Name,
			Custom: raw[i].Custom,
		}
		if raw[i].Schema != nil {
			f.Schema = raw[i].Schema.Type
		}
		fields = append(fields, f)
	}
	return fields, nil
}

// --- JQL passthrough ---

func (r *Repository) SearchJQL(ctx context.Context, jql string, limit int) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "search_jql", slog.String(adapterdriven.LogKeyQuery, jql))
	if limit <= 0 {
		limit = defaultLimit
	}
	return r.searchJQL(ctx, jql, limit)
}
