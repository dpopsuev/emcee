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
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
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
	ErrLinkNotFound        = errors.New("no matching link found")
)

// Compile-time interface compliance checks.
var (
	_ repository.IssueRepository        = (*Repository)(nil)
	_ repository.ProjectRepository      = (*Repository)(nil)
	_ repository.LabelRepository        = (*Repository)(nil)
	_ repository.CommentRepository      = (*Repository)(nil)
	_ repository.FieldRepository        = (*Repository)(nil)
	_ repository.JQLRepository          = (*Repository)(nil)
	_ repository.ExternalLinkRepository = (*Repository)(nil)
	_ repository.IssueLinkRepository    = (*Repository)(nil)
)

// Repository implements repository.IssueRepository for Jira.
type Repository struct {
	name         string
	baseURL      string
	email        string
	token        string
	project      string
	client       *http.Client
	mu           sync.RWMutex
	customFields map[string]string // display_name → field_id, populated from manifest + config
}

// New creates a Jira repository. configFields is the optional user-supplied mapping
// from config.Backend.Fields; nil is valid (discovery will run lazily).
func New(name, baseURL, email, token, project string, configFields map[string]string) (*Repository, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	cf := make(map[string]string, len(configFields))
	maps.Copy(cf, configFields)
	return &Repository{
		name:         name,
		baseURL:      baseURL,
		email:        email,
		token:        token,
		project:      project,
		client:       &http.Client{Timeout: defaultTimeout},
		customFields: cf,
	}, nil
}

// SetCustomFields hot-swaps the field manifest mapping on the live repository.
// Safe to call concurrently with ongoing requests.
func (r *Repository) SetCustomFields(fields map[string]string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.customFields = fields
}

var standardFields = strings.Split("summary,status,priority,assignee,reporter,description,labels,created,updated,project,issuetype,resolution,fixVersions,components,issuelinks,parent", ",")

func (r *Repository) Name() string { return r.name }

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
		limit, remaining, reset := infra.ParseRateLimitHeaders(resp.Header)
		return &infra.RateLimitError{
			Backend:    BackendName,
			RetryAfter: infra.ParseRetryAfter(resp.Header.Get("Retry-After")),
			Limit:      limit,
			Remaining:  remaining,
			Reset:      reset,
			Message:    string(respBody),
		}
	}
	if resp.StatusCode >= 400 {
		errMsg := parseJiraError(respBody)
		infra.LogAPIError(ctx, BackendName, method, path, resp.StatusCode, errMsg)
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
		return infra.SanitizeError(string(body))
	}

	parts := make([]string, 0, len(jiraErr.ErrorMessages)+len(jiraErr.Errors))
	parts = append(parts, jiraErr.ErrorMessages...)
	for field, msg := range jiraErr.Errors {
		parts = append(parts, fmt.Sprintf("%s: %s", field, msg))
	}
	if len(parts) == 0 {
		return infra.SanitizeError(string(body))
	}
	return strings.Join(parts, "; ")
}

// --- Issue operations ---

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	infra.LogOp(ctx, BackendName, "list", slog.String(infra.LogKeyProject, r.project))
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
	infra.LogOp(ctx, BackendName, "get", slog.String(infra.LogKeyIssueKey, key))
	var raw jiraIssue
	path := fmt.Sprintf("/rest/api/2/issue/%s", key)
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	r.mu.RLock()
	cf := r.customFields
	r.mu.RUnlock()
	issue := raw.toDomain(cf)
	return &issue, nil
}

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	infra.LogWrite(ctx, BackendName, "create", slog.String(infra.LogKeyProject, r.project), slog.String(infra.LogKeyTitle, input.Title))
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
	infra.LogWrite(ctx, BackendName, "update", slog.String(infra.LogKeyIssueKey, key))
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
	// Custom fields: resolve display name → fieldId, apply type coercion.
	r.mu.RLock()
	cf := r.customFields
	r.mu.RUnlock()
	for displayName, value := range input.CustomFields {
		fieldID, ok := cf[displayName]
		if !ok {
			continue // unmapped field, skip silently
		}
		fields[fieldID] = coerceCustomFieldValue(displayName, value)
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
	infra.LogOp(ctx, BackendName, "search", slog.String(infra.LogKeyQuery, query))
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
	infra.LogOp(ctx, BackendName, "list_children", slog.String(infra.LogKeyIssueKey, key))
	jql := fmt.Sprintf("parent = %s ORDER BY created ASC", key)
	return r.searchJQL(ctx, jql, defaultLimit)
}

// --- Project operations ---

func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	infra.LogOp(ctx, BackendName, "list_projects")
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
	infra.LogOp(ctx, BackendName, "list_labels")
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
	r.mu.RLock()
	cf := r.customFields
	r.mu.RUnlock()

	fields := make([]string, 0, len(cf)+len(standardFields))
	for _, id := range cf {
		fields = append(fields, id)
	}
	fields = append(fields, standardFields...)

	body := struct {
		JQL        string   `json:"jql"`
		MaxResults int      `json:"maxResults"`
		Fields     []string `json:"fields"`
	}{jql, limit, fields}

	var result struct {
		Issues []jiraIssue `json:"issues"`
	}
	if err := r.api(ctx, "POST", "/rest/api/3/search/jql", body, &result); err != nil {
		return nil, err
	}

	issues := make([]domain.Issue, 0, len(result.Issues))
	for i := range result.Issues {
		issues = append(issues, result.Issues[i].toDomain(cf))
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
	ID        string                     `json:"id"`
	Key       string                     `json:"key"`
	Self      string                     `json:"self"`
	RawFields map[string]json.RawMessage // populated by UnmarshalJSON
	Fields    struct {
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
		Reporter *struct {
			DisplayName string `json:"displayName"`
		} `json:"reporter"`
		Parent *struct {
			Key    string `json:"key"`
			Fields struct {
				Summary string `json:"summary"`
				Status  struct {
					Name string `json:"name"`
				} `json:"status"`
			} `json:"fields"`
		} `json:"parent"`
		FixVersions []struct {
			Name string `json:"name"`
		} `json:"fixVersions"`
		Components []struct {
			Name string `json:"name"`
		} `json:"components"`
		IssueLinks []struct {
			ID   string `json:"id"`
			Type struct {
				Name    string `json:"name"`
				Inward  string `json:"inward"`
				Outward string `json:"outward"`
			} `json:"type"`
			InwardIssue *struct {
				Key    string `json:"key"`
				Fields struct {
					Summary string `json:"summary"`
					Status  struct {
						Name string `json:"name"`
					} `json:"status"`
				} `json:"fields"`
			} `json:"inwardIssue"`
			OutwardIssue *struct {
				Key    string `json:"key"`
				Fields struct {
					Summary string `json:"summary"`
					Status  struct {
						Name string `json:"name"`
					} `json:"status"`
				} `json:"fields"`
			} `json:"outwardIssue"`
		} `json:"issuelinks"`
		Created string `json:"created"`
		Updated string `json:"updated"`
	} `json:"fields"`
}

type jiraProject struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

// UnmarshalJSON captures all fields into a raw map alongside the typed struct,
// allowing dynamic extraction of custom fields whose IDs vary per Jira instance.
func (j *jiraIssue) UnmarshalJSON(data []byte) error {
	type Alias jiraIssue
	if err := json.Unmarshal(data, (*Alias)(j)); err != nil {
		return err
	}
	// Capture the raw fields map for custom field extraction.
	var wrapper struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil {
		j.RawFields = wrapper.Fields
	}
	return nil
}

// Jira display names for fields that have typed representations in domain.Issue.
// These are matched against manifest keys (display_name → field_id) at runtime.
const (
	fieldDisplaySprint        = "Sprint"
	fieldDisplayStoryPoints   = "Story Points"
	fieldDisplayTargetVersion = "Target Version"
)

// handledByTypedField lists display names that map to typed Issue fields.
// Any manifest field NOT in this set falls through to Issue.CustomFields.
var handledByTypedField = map[string]bool{
	fieldDisplaySprint:        true,
	fieldDisplayStoryPoints:   true,
	fieldDisplayTargetVersion: true,
}

// applyCustomFields extracts all manifest-mapped fields from the raw field map.
// Fields with known display names populate typed Issue fields; all others go to
// Issue.CustomFields keyed by display name. Array-type fields (e.g. version lists)
// are joined as comma-separated strings.
func (j *jiraIssue) applyCustomFields(issue *domain.Issue, customFields map[string]string) {
	if sprintID, ok := customFields[fieldDisplaySprint]; ok {
		issue.Sprint = extractSprint(j.RawFields[sprintID])
	}
	if spID, ok := customFields[fieldDisplayStoryPoints]; ok {
		var v float64
		if raw := j.RawFields[spID]; raw != nil && json.Unmarshal(raw, &v) == nil {
			issue.StoryPoints = &v
		}
	}
	if tvID, ok := customFields[fieldDisplayTargetVersion]; ok {
		issue.TargetVersions = extractNameList(j.RawFields[tvID])
	}
	// All remaining manifest fields → CustomFields[display_name].
	for displayName, fieldID := range customFields {
		if handledByTypedField[displayName] {
			continue
		}
		raw, ok := j.RawFields[fieldID]
		if !ok || raw == nil || string(raw) == "null" {
			continue
		}
		// Try {name} array first (version-type fields).
		if names := extractNameList(raw); names != nil {
			if issue.CustomFields == nil {
				issue.CustomFields = make(map[string]string)
			}
			issue.CustomFields[displayName] = strings.Join(names, ", ")
			continue
		}
		var s string
		if json.Unmarshal(raw, &s) == nil && s != "" {
			if issue.CustomFields == nil {
				issue.CustomFields = make(map[string]string)
			}
			issue.CustomFields[displayName] = s
		}
	}
}

func extractSprint(raw json.RawMessage) string {
	if raw == nil {
		return ""
	}
	var sprints []struct {
		Name  string `json:"name"`
		State string `json:"state"`
	}
	if json.Unmarshal(raw, &sprints) != nil || len(sprints) == 0 {
		return ""
	}
	s := sprints[0].Name
	if sprints[0].State != "" {
		s += " (" + sprints[0].State + ")"
	}
	return s
}

func extractNameList(raw json.RawMessage) []string {
	if raw == nil {
		return nil
	}
	var items []struct {
		Name string `json:"name"`
	}
	if json.Unmarshal(raw, &items) != nil {
		return nil
	}
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}
	return names
}

// coerceCustomFieldValue converts a string value to the appropriate Jira API type
// based on the field display name.
func coerceCustomFieldValue(displayName, value string) any {
	switch displayName {
	case fieldDisplaySprint:
		var id int
		if _, err := fmt.Sscanf(value, "%d", &id); err == nil {
			return id
		}
	case fieldDisplayStoryPoints:
		var f float64
		if _, err := fmt.Sscanf(value, "%f", &f); err == nil {
			return f
		}
	case fieldDisplayTargetVersion:
		parts := strings.Split(value, ",")
		versions := make([]map[string]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				versions = append(versions, map[string]string{"name": t})
			}
		}
		return versions
	}
	return value
}

func (j *jiraIssue) toDomain(customFields map[string]string) domain.Issue {
	issue := domain.Issue{
		Ref:         BackendName + ":" + j.Key,
		ID:          j.ID,
		Key:         j.Key,
		Title:       j.Fields.Summary,
		Status:      mapStatusFromJira(j.Fields.Status.StatusCategory.Key),
		Labels:      j.Fields.Labels,
		Description: extractDescription(j.Fields.Description),
	}
	j.applyStandardFields(&issue)
	j.applyCustomFields(&issue, customFields)
	j.applyIssueLinks(&issue)
	if j.Self != "" {
		if idx := strings.Index(j.Self, "/rest/"); idx > 0 {
			issue.URL = j.Self[:idx] + "/browse/" + j.Key
		}
	}
	return issue
}

func (j *jiraIssue) applyStandardFields(issue *domain.Issue) {
	if j.Fields.Priority != nil {
		issue.Priority = mapPriorityFromJira(j.Fields.Priority.Name)
	}
	if j.Fields.Assignee != nil {
		issue.Assignee = j.Fields.Assignee.DisplayName
	}
	if j.Fields.Reporter != nil {
		issue.Reporter = j.Fields.Reporter.DisplayName
	}
	if j.Fields.Parent != nil {
		issue.Parent = &domain.IssueParent{
			Key:    j.Fields.Parent.Key,
			Title:  j.Fields.Parent.Fields.Summary,
			Status: j.Fields.Parent.Fields.Status.Name,
		}
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
	issue.CreatedAt, _ = time.Parse("2006-01-02T15:04:05.000+0000", j.Fields.Created)
	issue.UpdatedAt, _ = time.Parse("2006-01-02T15:04:05.000+0000", j.Fields.Updated)
}

func (j *jiraIssue) applyIssueLinks(issue *domain.Issue) {
	for _, link := range j.Fields.IssueLinks {
		if link.OutwardIssue != nil {
			issue.IssueLinks = append(issue.IssueLinks, domain.IssueLink{
				Type:         link.Type.Outward,
				Direction:    "outward",
				TargetRef:    BackendName + ":" + link.OutwardIssue.Key,
				TargetKey:    link.OutwardIssue.Key,
				TargetTitle:  link.OutwardIssue.Fields.Summary,
				TargetStatus: link.OutwardIssue.Fields.Status.Name,
			})
		}
		if link.InwardIssue != nil {
			issue.IssueLinks = append(issue.IssueLinks, domain.IssueLink{
				Type:         link.Type.Inward,
				Direction:    "inward",
				TargetRef:    BackendName + ":" + link.InwardIssue.Key,
				TargetKey:    link.InwardIssue.Key,
				TargetTitle:  link.InwardIssue.Fields.Summary,
				TargetStatus: link.InwardIssue.Fields.Status.Name,
			})
		}
	}
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
	infra.LogOp(ctx, BackendName, "list_comments", slog.String(infra.LogKeyIssueKey, key))
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
	infra.LogWrite(ctx, BackendName, "add_comment", slog.String(infra.LogKeyIssueKey, key))
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
	infra.LogOp(ctx, BackendName, "list_fields")
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
	infra.LogOp(ctx, BackendName, "search_jql", slog.String(infra.LogKeyQuery, jql))
	if limit <= 0 {
		limit = defaultLimit
	}
	return r.searchJQL(ctx, jql, limit)
}

// --- External link operations ---

type rpRemoteLink struct {
	Object struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	} `json:"object"`
	Application *struct {
		Name string `json:"name"`
	} `json:"application"`
}

func (r *Repository) ListExternalLinks(ctx context.Context, key string) ([]domain.ExternalLink, error) {
	infra.LogOp(ctx, BackendName, "list_external_links", slog.String(infra.LogKeyIssueKey, key))
	path := fmt.Sprintf("/rest/api/2/issue/%s/remotelink", key)
	var raw []rpRemoteLink
	if err := r.api(ctx, "GET", path, nil, &raw); err != nil {
		return nil, err
	}
	links := make([]domain.ExternalLink, 0, len(raw))
	for i := range raw {
		link := domain.ExternalLink{
			Title: raw[i].Object.Title,
			URL:   raw[i].Object.URL,
		}
		if raw[i].Application != nil {
			link.Type = raw[i].Application.Name
		}
		links = append(links, link)
	}
	return links, nil
}

// --- Issue link operations ---

func (r *Repository) CreateIssueLink(ctx context.Context, input domain.IssueLinkInput) error {
	linkType := input.Type
	if linkType == "" {
		linkType = "Related"
	}
	infra.LogWrite(ctx, BackendName, "create_issue_link",
		slog.String("type", linkType),
		slog.String("inward", input.InwardKey),
		slog.String("outward", input.OutwardKey))
	body := map[string]any{
		"type":         map[string]string{"name": linkType},
		"inwardIssue":  map[string]string{"key": input.InwardKey},
		"outwardIssue": map[string]string{"key": input.OutwardKey},
	}
	return r.api(ctx, "POST", "/rest/api/3/issueLink", body, nil)
}

func (r *Repository) DeleteIssueLink(ctx context.Context, inwardKey, outwardKey, linkType string) error {
	infra.LogWrite(ctx, BackendName, "delete_issue_link",
		slog.String("inward", inwardKey),
		slog.String("outward", outwardKey),
		slog.String("type", linkType))
	// Fetch the inward issue to find the matching link ID.
	var raw jiraIssue
	if err := r.api(ctx, "GET", fmt.Sprintf("/rest/api/3/issue/%s?fields=issuelinks", inwardKey), nil, &raw); err != nil {
		return err
	}
	for _, link := range raw.Fields.IssueLinks {
		if link.OutwardIssue != nil && link.OutwardIssue.Key == outwardKey &&
			(linkType == "" || strings.EqualFold(link.Type.Name, linkType)) {
			return r.api(ctx, "DELETE", "/rest/api/3/issueLink/"+link.ID, nil, nil)
		}
		if link.InwardIssue != nil && link.InwardIssue.Key == outwardKey &&
			(linkType == "" || strings.EqualFold(link.Type.Name, linkType)) {
			return r.api(ctx, "DELETE", "/rest/api/3/issueLink/"+link.ID, nil, nil)
		}
	}
	return fmt.Errorf("%w between %s and %s", ErrLinkNotFound, inwardKey, outwardKey)
}

func (r *Repository) ListLinkTypes(ctx context.Context) ([]domain.IssueLinkType, error) {
	infra.LogOp(ctx, BackendName, "list_link_types")
	var result struct {
		IssueLinkTypes []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Inward  string `json:"inward"`
			Outward string `json:"outward"`
		} `json:"issueLinkTypes"`
	}
	if err := r.api(ctx, "GET", "/rest/api/3/issueLinkType", nil, &result); err != nil {
		return nil, err
	}
	types := make([]domain.IssueLinkType, len(result.IssueLinkTypes))
	for i, t := range result.IssueLinkTypes {
		types[i] = domain.IssueLinkType{ID: t.ID, Name: t.Name, Inward: t.Inward, Outward: t.Outward}
	}
	return types, nil
}

// --- Delta sync ---

var _ repository.DeltaSyncer = (*Repository)(nil)

// ListUpdatedSince returns issues updated after since, scoped by the WatchScope.
// Projects, Labels, and IssueTypes are translated to JQL clauses.
// An empty scope returns nothing — callers must set at least one field.
func (r *Repository) ListUpdatedSince(ctx context.Context, since time.Time, scope domain.WatchScope, limit int) ([]domain.Issue, error) {
	infra.LogOp(ctx, BackendName, "list_updated_since")
	if scope.IsEmpty() {
		return nil, nil
	}
	if limit <= 0 {
		limit = defaultLimit
	}

	clauses := []string{fmt.Sprintf(`updated >= "%s"`, since.UTC().Format("2006-01-02 15:04"))}
	if len(scope.Projects) > 0 {
		quoted := make([]string, len(scope.Projects))
		for i, p := range scope.Projects {
			quoted[i] = `"` + p + `"`
		}
		clauses = append(clauses, "project IN ("+strings.Join(quoted, ",")+")")
	}
	if len(scope.IssueTypes) > 0 {
		quoted := make([]string, len(scope.IssueTypes))
		for i, t := range scope.IssueTypes {
			quoted[i] = `"` + t + `"`
		}
		clauses = append(clauses, "issuetype IN ("+strings.Join(quoted, ",")+")")
	}
	for _, label := range scope.Labels {
		clauses = append(clauses, `labels = "`+label+`"`)
	}

	jql := strings.Join(clauses, " AND ") + " ORDER BY updated ASC"
	return r.searchJQL(ctx, jql, limit)
}

// --- Changelog ---

var _ repository.ChangelogRepository = (*Repository)(nil)

func (r *Repository) ListChangelog(ctx context.Context, key string, limit int) ([]domain.ChangelogEntry, error) {
	infra.LogOp(ctx, BackendName, "list_changelog", slog.String(infra.LogKeyIssueKey, key))
	if limit <= 0 {
		limit = defaultLimit
	}
	path := fmt.Sprintf("/rest/api/3/issue/%s/changelog?maxResults=%d", key, limit)

	var result struct {
		Values []struct {
			ID     string `json:"id"`
			Author struct {
				DisplayName string `json:"displayName"`
			} `json:"author"`
			Created string `json:"created"`
			Items   []struct {
				Field     string `json:"field"`
				FieldID   string `json:"fieldId"`
				FromValue string `json:"fromString"`
				ToValue   string `json:"toString"`
			} `json:"items"`
		} `json:"values"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	entries := make([]domain.ChangelogEntry, 0, len(result.Values))
	for _, v := range result.Values {
		entry := domain.ChangelogEntry{
			ID:     v.ID,
			Author: v.Author.DisplayName,
		}
		entry.Created, _ = time.Parse("2006-01-02T15:04:05.000-0700", v.Created)
		for _, item := range v.Items {
			entry.Items = append(entry.Items, domain.ChangelogItem{
				Field:     item.Field,
				FieldID:   item.FieldID,
				FromValue: item.FromValue,
				ToValue:   item.ToValue,
			})
		}
		entries = append(entries, entry)
	}
	return entries, nil
}
