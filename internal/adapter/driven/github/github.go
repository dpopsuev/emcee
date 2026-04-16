// Package github implements the driven (outbound) adapter for GitHub's REST API v3.
package github

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
	"strconv"
	"strings"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

const (
	apiURL      = "https://api.github.com"
	BackendName = "github"

	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
)

var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrCreateFailed  = errors.New("issue creation failed")
	ErrRepoEmpty     = errors.New("repository owner/name is required")
	ErrAPIError      = errors.New("github API error")
	ErrNotAnIssue    = errors.New("not an issue")
	ErrProjectCreate = errors.New("project creation not yet supported (requires GitHub Projects v2 GraphQL API)")
	ErrProjectUpdate = errors.New("project update not yet supported (requires GitHub Projects v2 GraphQL API)")
)

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository   = (*Repository)(nil)
	_ driven.ProjectRepository = (*Repository)(nil)
	_ driven.LabelRepository   = (*Repository)(nil)
	_ driven.CommentRepository = (*Repository)(nil)
	_ driven.PRRepository      = (*Repository)(nil)
)

// Repository implements driven.IssueRepository for GitHub.
type Repository struct {
	baseURL string
	token   string
	owner   string
	repo    string
	client  *http.Client
}

// New creates a GitHub repository.
func New(token, owner, repo string) (*Repository, error) {
	return NewWithURL(token, owner, repo, apiURL)
}

// NewWithURL creates a GitHub repository with a custom API URL (for testing).
func NewWithURL(token, owner, repo, url string) (*Repository, error) {
	if owner == "" || repo == "" {
		return nil, ErrRepoEmpty
	}
	return &Repository{
		baseURL: strings.TrimRight(url, "/"),
		token:   token,
		owner:   owner,
		repo:    repo,
		client:  &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (r *Repository) Name() string { return BackendName }

// api makes an authenticated request to the GitHub REST API.
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
	req.Header.Set("Authorization", "token "+r.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/vnd.github.v3+json")

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
		sanitized := adapterdriven.SanitizeError(string(respBody))
		adapterdriven.LogAPIError(ctx, BackendName, method, path, resp.StatusCode, sanitized)
		return fmt.Errorf("%w %s %s: %d: %s", ErrAPIError, method, path, resp.StatusCode, sanitized)
	}

	if result != nil {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// --- Issue operations ---

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "list")
	path := fmt.Sprintf("/repos/%s/%s/issues?per_page=%d&state=all", r.owner, r.repo, defaultLimit)

	if filter.Limit > 0 && filter.Limit < defaultLimit {
		path = fmt.Sprintf("/repos/%s/%s/issues?per_page=%d&state=all", r.owner, r.repo, filter.Limit)
	}

	// Add state filter
	if filter.Status != "" {
		state := mapStatusToGitHub(filter.Status)
		path = strings.Replace(path, "state=all", "state="+state, 1)
	}

	// Add assignee filter
	if filter.Assignee != "" {
		path += "&assignee=" + filter.Assignee
	}

	// Add labels filter
	if len(filter.Labels) > 0 {
		path += "&labels=" + strings.Join(filter.Labels, ",")
	}

	var raw []githubIssue
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(raw))
	for i := range raw {
		// Skip pull requests (GitHub returns them in issues endpoint)
		if raw[i].PullRequest != nil {
			continue
		}
		issues = append(issues, raw[i].toDomain())
	}
	return issues, nil
}

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "get", slog.String(adapterdriven.LogKeyIssueKey, key))
	// key can be either issue number or "owner/repo#number" format
	number := r.parseIssueNumber(key)

	var raw githubIssue
	path := fmt.Sprintf("/repos/%s/%s/issues/%s", r.owner, r.repo, number)
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	if raw.PullRequest != nil {
		return nil, fmt.Errorf("%w: #%s is a pull request", ErrNotAnIssue, number)
	}

	issue := raw.toDomain()
	return &issue, nil
}

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "create", slog.String(adapterdriven.LogKeyTitle, input.Title))
	body := map[string]any{
		"title": input.Title,
		"body":  input.Description,
	}

	if len(input.Labels) > 0 {
		body["labels"] = input.Labels
	}

	if input.Assignee != "" {
		body["assignees"] = []string{input.Assignee}
	}

	var result githubIssue
	path := fmt.Sprintf("/repos/%s/%s/issues", r.owner, r.repo)
	if err := r.api(ctx, "POST", path, body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateFailed, err)
	}

	issue := result.toDomain()
	return &issue, nil
}

func (r *Repository) Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "update", slog.String(adapterdriven.LogKeyIssueKey, key))
	number := r.parseIssueNumber(key)
	body := map[string]any{}

	if input.Title != nil {
		body["title"] = *input.Title
	}
	if input.Description != nil {
		body["body"] = *input.Description
	}
	if input.Status != nil {
		body["state"] = mapStatusToGitHub(*input.Status)
	}
	if input.Labels != nil {
		body["labels"] = input.Labels
	}
	if input.Assignee != nil {
		if *input.Assignee == "" {
			body["assignees"] = []string{}
		} else {
			body["assignees"] = []string{*input.Assignee}
		}
	}

	var result githubIssue
	path := fmt.Sprintf("/repos/%s/%s/issues/%s", r.owner, r.repo, number)
	if err := r.api(ctx, "PATCH", path, body, &result); err != nil {
		return nil, err
	}

	issue := result.toDomain()
	return &issue, nil
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "search", slog.String(adapterdriven.LogKeyQuery, query))
	if limit <= 0 {
		limit = defaultLimit
	}

	// GitHub search API requires a different format
	searchQuery := fmt.Sprintf("repo:%s/%s %s", r.owner, r.repo, query)
	path := fmt.Sprintf("/search/issues?q=%s&per_page=%d", searchQuery, limit)

	var result struct {
		Items []githubIssue `json:"items"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(result.Items))
	for i := range result.Items {
		if result.Items[i].PullRequest != nil {
			continue
		}
		issues = append(issues, result.Items[i].toDomain())
	}
	return issues, nil
}

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_children", slog.String(adapterdriven.LogKeyIssueKey, key))
	// GitHub doesn't have native sub-issues, return empty list
	// Could be enhanced to parse issue body for task lists or related issues
	return []domain.Issue{}, nil
}

// --- Project operations ---

func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_projects")
	path := fmt.Sprintf("/repos/%s/%s/projects", r.owner, r.repo)

	var raw []githubProject
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	projects := make([]domain.Project, 0, len(raw))
	for i, gp := range raw {
		if i >= limit {
			break
		}
		projects = append(projects, domain.Project{
			ID:   strconv.Itoa(gp.ID),
			Name: gp.Name,
			URL:  gp.HTMLURL,
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
	path := fmt.Sprintf("/repos/%s/%s/labels", r.owner, r.repo)

	var raw []githubLabel
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	labels := make([]domain.Label, 0, len(raw))
	for _, gl := range raw {
		labels = append(labels, domain.Label{
			ID:   strconv.Itoa(gl.ID),
			Name: gl.Name,
		})
	}
	return labels, nil
}

func (r *Repository) CreateLabel(ctx context.Context, input domain.LabelCreateInput) (*domain.Label, error) {
	adapterdriven.LogWrite(ctx, BackendName, "create_label", slog.String(adapterdriven.LogKeyName, input.Name))
	body := map[string]any{
		"name":  input.Name,
		"color": "ededed", // default gray
	}

	var result githubLabel
	path := fmt.Sprintf("/repos/%s/%s/labels", r.owner, r.repo)
	if err := r.api(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}

	return &domain.Label{
		ID:   strconv.Itoa(result.ID),
		Name: result.Name,
	}, nil
}

// --- Internal helpers ---

func (r *Repository) parseIssueNumber(key string) string {
	// Handle formats: "123", "#123", "owner/repo#123"
	key = strings.TrimPrefix(key, "#")
	if idx := strings.LastIndex(key, "#"); idx >= 0 {
		key = key[idx+1:]
	}
	return key
}

// --- GitHub API types ---

type githubIssue struct {
	Number      int           `json:"number"`
	Title       string        `json:"title"`
	Body        string        `json:"body"`
	State       string        `json:"state"`
	HTMLURL     string        `json:"html_url"`
	User        *githubUser   `json:"user"`
	Assignee    *githubUser   `json:"assignee"`
	Labels      []githubLabel `json:"labels"`
	CreatedAt   string        `json:"created_at"`
	UpdatedAt   string        `json:"updated_at"`
	PullRequest *struct{}     `json:"pull_request,omitempty"`
}

type githubUser struct {
	Login string `json:"login"`
}

type githubLabel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type githubProject struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	HTMLURL string `json:"html_url"`
}

func (gh githubIssue) toDomain() domain.Issue {
	issue := domain.Issue{
		Ref:         fmt.Sprintf("%s:%s#%d", BackendName, "", gh.Number),
		ID:          strconv.Itoa(gh.Number),
		Key:         fmt.Sprintf("#%d", gh.Number),
		Title:       gh.Title,
		Description: gh.Body,
		Status:      mapStatusFromGitHub(gh.State),
		URL:         gh.HTMLURL,
	}

	if gh.Assignee != nil {
		issue.Assignee = gh.Assignee.Login
	}

	if len(gh.Labels) > 0 {
		issue.Labels = make([]string, 0, len(gh.Labels))
		for _, l := range gh.Labels {
			issue.Labels = append(issue.Labels, l.Name)
		}
	}

	// Parse priority from labels (if present)
	for _, l := range gh.Labels {
		if p := parsePriorityFromLabel(l.Name); p != domain.PriorityNone {
			issue.Priority = p
			break
		}
	}

	if gh.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, gh.CreatedAt); err == nil {
			issue.CreatedAt = t
		}
	}
	if gh.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, gh.UpdatedAt); err == nil {
			issue.UpdatedAt = t
		}
	}

	return issue
}

// --- Status mapping ---

func mapStatusFromGitHub(state string) domain.Status {
	switch strings.ToLower(state) {
	case "open":
		return domain.StatusTodo
	case "closed":
		return domain.StatusDone
	default:
		return domain.StatusTodo
	}
}

func mapStatusToGitHub(status domain.Status) string {
	switch status {
	case domain.StatusDone, domain.StatusCanceled:
		return "closed"
	default:
		return "open"
	}
}

// --- Comment operations ---

type githubComment struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	User      *struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (gc githubComment) toDomain() domain.Comment {
	c := domain.Comment{
		ID:   fmt.Sprintf("%d", gc.ID),
		Body: gc.Body,
	}
	if gc.User != nil {
		c.Author = gc.User.Login
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, gc.CreatedAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, gc.UpdatedAt)
	return c
}

func (r *Repository) ListComments(ctx context.Context, key string) ([]domain.Comment, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_comments", slog.String(adapterdriven.LogKeyIssueKey, key))
	number := r.parseIssueNumber(key)
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", r.owner, r.repo, number)
	var raw []githubComment
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	comments := make([]domain.Comment, len(raw))
	for i := range raw {
		comments[i] = raw[i].toDomain()
	}
	return comments, nil
}

func (r *Repository) AddComment(ctx context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error) {
	adapterdriven.LogWrite(ctx, BackendName, "add_comment", slog.String(adapterdriven.LogKeyIssueKey, key))
	number := r.parseIssueNumber(key)
	path := fmt.Sprintf("/repos/%s/%s/issues/%s/comments", r.owner, r.repo, number)
	body := map[string]string{"body": input.Body}
	var raw githubComment
	if err := r.api(ctx, "POST", path, body, &raw); err != nil {
		return nil, err
	}
	c := raw.toDomain()
	return &c, nil
}

// --- Priority mapping (via labels) ---

func parsePriorityFromLabel(label string) domain.Priority {
	lower := strings.ToLower(label)
	switch {
	case strings.Contains(lower, "urgent"), strings.Contains(lower, "critical"):
		return domain.PriorityUrgent
	case strings.Contains(lower, "high"):
		return domain.PriorityHigh
	case strings.Contains(lower, "medium"):
		return domain.PriorityMedium
	case strings.Contains(lower, "low"):
		return domain.PriorityLow
	default:
		return domain.PriorityNone
	}
}

// --- Pull Request operations ---

type githubPR struct {
	Number   int    `json:"number"`
	Title    string `json:"title"`
	State    string `json:"state"`
	MergedAt string `json:"merged_at"`
	HTMLURL  string `json:"html_url"`
	User     *struct {
		Login string `json:"login"`
	} `json:"user"`
}

func (pr *githubPR) toDomain(repo string) domain.PullRequest {
	p := domain.PullRequest{
		Number: pr.Number,
		Title:  pr.Title,
		State:  pr.State,
		URL:    pr.HTMLURL,
		Repo:   repo,
	}
	if pr.User != nil {
		p.Author = pr.User.Login
	}
	if pr.MergedAt != "" {
		p.State = "merged"
		p.MergedAt, _ = time.Parse(time.RFC3339, pr.MergedAt)
	}
	return p
}

func (r *Repository) ListPRs(ctx context.Context, filter domain.PRFilter) ([]domain.PullRequest, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_prs")
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	// Use search API for date filtering
	// Allow repo override via filter for multi-project queries
	owner, repo := r.owner, r.repo
	if filter.Repo != "" {
		parts := strings.SplitN(filter.Repo, "/", 2)
		if len(parts) == 2 {
			owner, repo = parts[0], parts[1]
		}
	}
	q := fmt.Sprintf("repo:%s/%s is:pr", owner, repo)
	if filter.Author != "" {
		q += " author:" + filter.Author
	}
	if filter.State == "merged" || filter.State == "" {
		q += " is:merged"
	} else {
		q += " is:" + filter.State
	}
	if filter.MergedAfter != "" {
		q += " merged:>=" + filter.MergedAfter
	}
	if filter.MergedBefore != "" {
		q += " merged:<" + filter.MergedBefore
	}

	path := fmt.Sprintf("/search/issues?q=%s&per_page=%d&sort=updated&order=desc", url.QueryEscape(q), limit)
	var result struct {
		Items []githubPR `json:"items"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	repoName := owner + "/" + repo
	prs := make([]domain.PullRequest, 0, len(result.Items))
	for i := range result.Items {
		prs = append(prs, result.Items[i].toDomain(repoName))
	}
	return prs, nil
}
