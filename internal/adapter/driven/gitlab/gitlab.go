// Package gitlab implements the driven (outbound) adapter for GitLab's REST API v4.
package gitlab

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
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
	defaultURL     = "https://gitlab.com"
	BackendName    = "gitlab"
	defaultTimeout = 30 * time.Second
	defaultLimit   = 50

	// URL validation constants.
	schemeHTTPS   = "https"
	schemeHTTP    = "http"
	localhostHost = "localhost"

	// Private IP CIDR ranges (RFC1918).
	cidr10Private  = "10.0.0.0/8"
	cidr172Private = "172.16.0.0/12"
	cidr192Private = "192.168.0.0/16"

	// IPv6 ULA ranges.
	cidrIPv6ULA1 = "fc00::/7"
	cidrIPv6ULA2 = "fd00::/8"
)

var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrCreateFailed  = errors.New("issue creation failed")
	ErrProjectEmpty  = errors.New("project ID is required")
	ErrInvalidURL    = errors.New("invalid GitLab URL")
	ErrAPIError      = errors.New("gitlab API error")
	ErrProjectCreate = errors.New("project creation not yet supported")
	ErrProjectUpdate = errors.New("project update not yet supported")
)

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository   = (*Repository)(nil)
	_ driven.ProjectRepository = (*Repository)(nil)
	_ driven.LabelRepository   = (*Repository)(nil)
	_ driven.CommentRepository = (*Repository)(nil)
	_ driven.PRRepository      = (*Repository)(nil)
)

// Repository implements driven.IssueRepository for GitLab.
type Repository struct {
	name      string
	baseURL   string
	token     string
	projectID string // numeric ID or "namespace/project" format
	client    *http.Client
}

// New creates a GitLab repository (defaults to gitlab.com).
func New(name, token, projectID string) (*Repository, error) {
	return NewWithURL(name, token, projectID, defaultURL)
}

// NewWithURL creates a GitLab repository with a custom URL (for self-hosted instances).
func NewWithURL(name, token, projectID, baseURL string) (*Repository, error) {
	if projectID == "" {
		return nil, ErrProjectEmpty
	}
	if baseURL == "" {
		baseURL = defaultURL
	}

	// Validate URL to prevent SSRF attacks
	if err := validateURL(baseURL); err != nil {
		return nil, err
	}

	return &Repository{
		name:      name,
		baseURL:   strings.TrimRight(baseURL, "/"),
		token:     token,
		projectID: url.PathEscape(projectID), // GitLab accepts URL-encoded project IDs
		client:    &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (r *Repository) Name() string { return r.name }

// validateURL checks if the GitLab URL is safe to use (SSRF prevention).
// Blocks private IP ranges and enforces HTTPS (except for localhost dev).
func validateURL(rawURL string) error {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: invalid URL format: %w", ErrInvalidURL, err)
	}

	// Validate scheme
	if parsed.Scheme != schemeHTTPS && parsed.Scheme != schemeHTTP {
		return fmt.Errorf("%w: scheme must be https:// or http:// (got %s://)", ErrInvalidURL, parsed.Scheme)
	}

	// Allow http://localhost for development
	if parsed.Scheme == schemeHTTP && parsed.Hostname() != localhostHost {
		return fmt.Errorf("%w: http:// only allowed for localhost (got %s). Use https:// for remote instances", ErrInvalidURL, parsed.Hostname())
	}

	// Extract hostname
	hostname := parsed.Hostname()

	// Check if hostname is an IP address
	ip := net.ParseIP(hostname)
	if ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("%w: private IP addresses are not allowed (blocks SSRF). Use a public domain or localhost for development", ErrInvalidURL)
		}
	}

	return nil
}

// isPrivateIP returns true if the IP is in a private range (SSRF protection).
func isPrivateIP(ip net.IP) bool {
	// Check for loopback (127.0.0.0/8, ::1)
	if ip.IsLoopback() {
		return true
	}

	// Check for link-local (169.254.0.0/16, fe80::/10)
	if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}

	// Check for private ranges
	privateRanges := []string{
		cidr10Private,
		cidr172Private,
		cidr192Private,
		cidrIPv6ULA1,
		cidrIPv6ULA2,
	}

	for _, cidr := range privateRanges {
		_, network, _ := net.ParseCIDR(cidr)
		if network != nil && network.Contains(ip) {
			return true
		}
	}

	return false
}

// api makes an authenticated request to the GitLab REST API.
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
	req.Header.Set("PRIVATE-TOKEN", r.token)
	req.Header.Set("Content-Type", "application/json")

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
	path := fmt.Sprintf("/api/v4/projects/%s/issues?per_page=%d", r.projectID, defaultLimit)

	if filter.Limit > 0 && filter.Limit < defaultLimit {
		path = fmt.Sprintf("/api/v4/projects/%s/issues?per_page=%d", r.projectID, filter.Limit)
	}

	// Add state filter
	if filter.Status != "" {
		state := mapStatusToGitLab(filter.Status)
		path += "&state=" + state
	}

	// Add assignee filter
	if filter.Assignee != "" {
		path += "&assignee_username=" + filter.Assignee
	}

	// Add labels filter
	if len(filter.Labels) > 0 {
		path += "&labels=" + strings.Join(filter.Labels, ",")
	}

	var raw []gitlabIssue
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(raw))
	for i := range raw {
		issues = append(issues, raw[i].toDomain())
	}
	return issues, nil
}

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "get", slog.String(adapterdriven.LogKeyIssueKey, key))
	// key can be issue IID (internal ID per project)
	iid := r.parseIssueIID(key)

	var raw gitlabIssue
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%s", r.projectID, iid)
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	issue := raw.toDomain()
	return &issue, nil
}

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "create", slog.String(adapterdriven.LogKeyTitle, input.Title))
	body := map[string]any{
		"title":       input.Title,
		"description": input.Description,
	}

	if len(input.Labels) > 0 {
		body["labels"] = strings.Join(input.Labels, ",")
	}

	// NOTE: GitLab uses assignee_ids (numeric), which requires resolving
	// username to ID first. Skipped for now; can be added in Update.
	// if input.Assignee != "" { ... }

	var result gitlabIssue
	path := fmt.Sprintf("/api/v4/projects/%s/issues", r.projectID)
	if err := r.api(ctx, "POST", path, body, &result); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrCreateFailed, err)
	}

	issue := result.toDomain()
	return &issue, nil
}

func (r *Repository) Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	adapterdriven.LogWrite(ctx, BackendName, "update", slog.String(adapterdriven.LogKeyIssueKey, key))
	iid := r.parseIssueIID(key)
	body := map[string]any{}

	if input.Title != nil {
		body["title"] = *input.Title
	}
	if input.Description != nil {
		body["description"] = *input.Description
	}
	if input.Status != nil {
		body["state_event"] = mapStatusEventToGitLab(*input.Status)
	}
	if input.Labels != nil {
		body["labels"] = strings.Join(input.Labels, ",")
	}

	var result gitlabIssue
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%s", r.projectID, iid)
	if err := r.api(ctx, "PUT", path, body, &result); err != nil {
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

	// GitLab search API
	path := fmt.Sprintf("/api/v4/projects/%s/issues?search=%s&per_page=%d",
		r.projectID, url.QueryEscape(query), limit)

	var raw []gitlabIssue
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(raw))
	for i := range raw {
		issues = append(issues, raw[i].toDomain())
	}
	return issues, nil
}

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_children", slog.String(adapterdriven.LogKeyIssueKey, key))
	// GitLab doesn't have native sub-issues, return empty list
	// Could be enhanced to use issue links or related issues
	return []domain.Issue{}, nil
}

// --- Project operations ---

func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_projects")
	path := "/api/v4/projects?membership=true&per_page=20"

	var raw []gitlabProject
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
			URL:  gp.WebURL,
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
	path := fmt.Sprintf("/api/v4/projects/%s/labels", r.projectID)

	var raw []gitlabLabel
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
		"color": "#428BCA", // default blue
	}

	var result gitlabLabel
	path := fmt.Sprintf("/api/v4/projects/%s/labels", r.projectID)
	if err := r.api(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}

	return &domain.Label{
		ID:   strconv.Itoa(result.ID),
		Name: result.Name,
	}, nil
}

// --- Internal helpers ---

func (r *Repository) parseIssueIID(key string) string {
	// Handle formats: "123", "#123"
	key = strings.TrimPrefix(key, "#")
	return key
}

// --- GitLab API types ---

type gitlabIssue struct {
	ID          int         `json:"id"`
	IID         int         `json:"iid"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	State       string      `json:"state"`
	WebURL      string      `json:"web_url"`
	Author      *gitlabUser `json:"author"`
	Assignee    *gitlabUser `json:"assignee"`
	Labels      []string    `json:"labels"`
	CreatedAt   string      `json:"created_at"`
	UpdatedAt   string      `json:"updated_at"`
}

type gitlabUser struct {
	Username string `json:"username"`
	Name     string `json:"name"`
}

type gitlabLabel struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type gitlabProject struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	WebURL string `json:"web_url"`
}

func (gl gitlabIssue) toDomain() domain.Issue {
	issue := domain.Issue{
		Ref:         fmt.Sprintf("%s:%d", BackendName, gl.IID),
		ID:          strconv.Itoa(gl.IID),
		Key:         fmt.Sprintf("#%d", gl.IID),
		Title:       gl.Title,
		Description: gl.Description,
		Status:      mapStatusFromGitLab(gl.State),
		URL:         gl.WebURL,
	}

	if gl.Assignee != nil {
		issue.Assignee = gl.Assignee.Username
	}

	if len(gl.Labels) > 0 {
		issue.Labels = gl.Labels
	}

	// Parse priority from labels (if present)
	for _, l := range gl.Labels {
		if p := parsePriorityFromLabel(l); p != domain.PriorityNone {
			issue.Priority = p
			break
		}
	}

	if gl.CreatedAt != "" {
		if t, err := time.Parse(time.RFC3339, gl.CreatedAt); err == nil {
			issue.CreatedAt = t
		}
	}
	if gl.UpdatedAt != "" {
		if t, err := time.Parse(time.RFC3339, gl.UpdatedAt); err == nil {
			issue.UpdatedAt = t
		}
	}

	return issue
}

// --- Status mapping ---

func mapStatusFromGitLab(state string) domain.Status {
	switch strings.ToLower(state) {
	case "opened":
		return domain.StatusTodo
	case "closed":
		return domain.StatusDone
	default:
		return domain.StatusTodo
	}
}

func mapStatusToGitLab(status domain.Status) string {
	switch status {
	case domain.StatusDone, domain.StatusCanceled:
		return "closed"
	default:
		return "opened"
	}
}

func mapStatusEventToGitLab(status domain.Status) string {
	// GitLab uses state_event field for transitions
	switch status {
	case domain.StatusDone, domain.StatusCanceled:
		return "close"
	default:
		return "reopen"
	}
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

// --- Comment operations ---

type gitlabNote struct {
	ID        int    `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Author    struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (gn gitlabNote) toDomain() domain.Comment {
	c := domain.Comment{
		ID:     fmt.Sprintf("%d", gn.ID),
		Body:   gn.Body,
		Author: gn.Author.Username,
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, gn.CreatedAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, gn.UpdatedAt)
	return c
}

func (r *Repository) ListComments(ctx context.Context, key string) ([]domain.Comment, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_comments", slog.String(adapterdriven.LogKeyIssueKey, key))
	iid := r.parseIssueIID(key)
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%s/notes", r.projectID, iid)
	var raw []gitlabNote
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
	iid := r.parseIssueIID(key)
	path := fmt.Sprintf("/api/v4/projects/%s/issues/%s/notes", r.projectID, iid)
	body := map[string]string{"body": input.Body}
	var raw gitlabNote
	if err := r.api(ctx, "POST", path, body, &raw); err != nil {
		return nil, err
	}
	c := raw.toDomain()
	return &c, nil
}

// --- Merge Request operations ---

type gitlabMR struct {
	IID      int    `json:"iid"`
	Title    string `json:"title"`
	State    string `json:"state"`
	MergedAt string `json:"merged_at"`
	WebURL   string `json:"web_url"`
	Author   struct {
		Username string `json:"username"`
	} `json:"author"`
}

func (mr *gitlabMR) toDomain(projectID string) domain.PullRequest {
	p := domain.PullRequest{
		Number: mr.IID,
		Title:  mr.Title,
		State:  mr.State,
		URL:    mr.WebURL,
		Author: mr.Author.Username,
		Repo:   projectID,
	}
	if mr.MergedAt != "" {
		p.MergedAt, _ = time.Parse(time.RFC3339, mr.MergedAt)
	}
	return p
}

func (r *Repository) ListPRs(ctx context.Context, filter domain.PRFilter) ([]domain.PullRequest, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_prs")
	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}

	// Allow project override via filter for multi-project queries
	projectID := r.projectID
	if filter.Repo != "" {
		projectID = url.PathEscape(filter.Repo)
	}
	path := fmt.Sprintf("/api/v4/projects/%s/merge_requests?per_page=%d&order_by=updated_at&sort=desc", projectID, limit)
	if filter.State != "" {
		path += "&state=" + filter.State
	} else {
		path += "&state=merged"
	}
	if filter.Author != "" {
		path += "&author_username=" + filter.Author
	}
	if filter.MergedAfter != "" {
		path += "&created_after=" + filter.MergedAfter + "T00:00:00Z"
	}
	if filter.MergedBefore != "" {
		path += "&created_before=" + filter.MergedBefore + "T23:59:59Z"
	}

	var raw []gitlabMR
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}

	prs := make([]domain.PullRequest, 0, len(raw))
	for i := range raw {
		prs = append(prs, raw[i].toDomain(r.projectID))
	}
	return prs, nil
}
