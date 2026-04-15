// Package linear implements the driven (outbound) adapter for Linear's GraphQL API.
package linear

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

const (
	apiURL = "https://api.linear.app/graphql"

	// Backend name constant.
	BackendName = "linear"

	// Linear state type constants.
	stateBacklog   = "backlog"
	stateUnstarted = "unstarted"
	stateStarted   = "started"
	stateCompleted = "completed"
	stateCanceled  = "canceled"

	// Log keys.
	logKeyBackend   = "backend"
	logKeyTeam      = "team"
	logKeyOperation = "op"
	logKeyIssueKey  = "key"
	logKeyTitle     = "title"
	logKeyName      = "name"
	logKeyQuery     = "query"
	logKeyID        = "id"
	logKeyCount     = "count"

	// Rate limit detection strings (GraphQL errors).
	errMsgRateLimit       = "rate limit"
	errMsgTooManyRequests = "too many requests"

	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
)

// Sentinel errors.
var (
	ErrIssueNotFound       = errors.New("issue not found")
	ErrCreateFailed        = errors.New("issue creation failed")
	ErrTeamNotFound        = errors.New("team not found")
	ErrGraphQL             = errors.New("graphql error")
	ErrNoStateMatch        = errors.New("no state matching")
	ErrDocumentCreate      = errors.New("document creation failed")
	ErrProjectCreate       = errors.New("project creation failed")
	ErrProjectUpdate       = errors.New("project update failed")
	ErrInitiativeCreate    = errors.New("initiative creation failed")
	ErrLabelCreate         = errors.New("label creation failed")
	ErrBulkCreate          = errors.New("bulk issue creation failed")
	ErrUserNotFound        = errors.New("user not found")
	ErrCommentCreateFailed = errors.New("failed to create comment")
)

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository      = (*Repository)(nil)
	_ driven.DocumentRepository   = (*Repository)(nil)
	_ driven.ProjectRepository    = (*Repository)(nil)
	_ driven.InitiativeRepository = (*Repository)(nil)
	_ driven.LabelRepository      = (*Repository)(nil)
	_ driven.BulkIssueRepository  = (*Repository)(nil)
	_ driven.UserResolver         = (*Repository)(nil)
	_ driven.CommentRepository    = (*Repository)(nil)
)

// Repository implements driven.IssueRepository for Linear.
type Repository struct {
	apiKey string
	teamID string
	team   string
	url    string
	client *http.Client
}

// New creates a Linear repository. It resolves the team key to an ID on init.
func New(apiKey, teamKey string) (*Repository, error) {
	return NewWithURL(apiKey, teamKey, apiURL)
}

// NewWithURL creates a Linear repository with a custom API URL (for testing).
func NewWithURL(apiKey, teamKey, url string) (*Repository, error) {
	r := &Repository{
		apiKey: apiKey,
		team:   teamKey,
		url:    url,
		client: &http.Client{Timeout: defaultTimeout},
	}
	teamID, err := r.resolveTeam(context.Background(), teamKey)
	if err != nil {
		return nil, fmt.Errorf("resolve team %q: %w", teamKey, err)
	}
	r.teamID = teamID
	return r, nil
}

func (r *Repository) Name() string { return BackendName }

func (r *Repository) gql(ctx context.Context, query string, result any) error {
	body, _ := json.Marshal(map[string]string{"query": query})
	req, err := http.NewRequestWithContext(ctx, "POST", r.url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", r.apiKey)

	resp, err := r.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Check for HTTP 429 before reading body
	if resp.StatusCode == http.StatusTooManyRequests {
		limit, remaining, reset := adapterdriven.ParseRateLimitHeaders(resp.Header)
		return &adapterdriven.RateLimitError{
			Backend:    BackendName,
			RetryAfter: adapterdriven.ParseRetryAfter(resp.Header.Get("Retry-After")),
			Limit:      limit,
			Remaining:  remaining,
			Reset:      reset,
		}
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	var gqlResp struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(data, &gqlResp); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	if len(gqlResp.Errors) > 0 {
		errMsg := gqlResp.Errors[0].Message
		// Check if GraphQL error indicates rate limiting
		errMsgLower := strings.ToLower(errMsg)
		if strings.Contains(errMsgLower, errMsgRateLimit) ||
			strings.Contains(errMsgLower, errMsgTooManyRequests) {
			return &adapterdriven.RateLimitError{
				Backend: BackendName,
				Message: errMsg,
			}
		}
		sanitized := adapterdriven.SanitizeError(errMsg)
		return fmt.Errorf("%w: %s", ErrGraphQL, sanitized)
	}
	if result != nil {
		return json.Unmarshal(gqlResp.Data, result)
	}
	return nil
}

func (r *Repository) resolveTeam(ctx context.Context, key string) (string, error) {
	var result struct {
		Teams struct {
			Nodes []struct {
				ID  string `json:"id"`
				Key string `json:"key"`
			} `json:"nodes"`
		} `json:"teams"`
	}
	if err := r.gql(ctx, `{ teams { nodes { id key } } }`, &result); err != nil {
		return "", err
	}
	for _, t := range result.Teams.Nodes {
		if strings.EqualFold(t.Key, key) {
			return t.ID, nil
		}
	}
	return "", fmt.Errorf("%w: %s", ErrTeamNotFound, key)
}

type linearIssue struct {
	ID         string `json:"id"`
	Identifier string `json:"identifier"`
	Title      string `json:"title"`
	Desc       string `json:"description"`
	Priority   int    `json:"priority"`
	URL        string `json:"url"`
	CreatedAt  string `json:"createdAt"`
	UpdatedAt  string `json:"updatedAt"`
	State      struct {
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"state"`
	Assignee *struct {
		Name string `json:"name"`
	} `json:"assignee"`
	Labels struct {
		Nodes []struct {
			Name string `json:"name"`
		} `json:"nodes"`
	} `json:"labels"`
}

const issueFields = `id identifier title description priority url createdAt updatedAt
	state { name type } assignee { name } labels { nodes { name } }`

func (li *linearIssue) toDomain() domain.Issue {
	issue := domain.Issue{
		Ref:         "linear:" + li.Identifier,
		ID:          li.ID,
		Key:         li.Identifier,
		Title:       li.Title,
		Description: li.Desc,
		Status:      mapStatus(li.State.Type),
		Priority:    domain.Priority(li.Priority),
		URL:         li.URL,
	}
	if li.Assignee != nil {
		issue.Assignee = li.Assignee.Name
	}
	for _, l := range li.Labels.Nodes {
		issue.Labels = append(issue.Labels, l.Name)
	}
	issue.CreatedAt, _ = time.Parse(time.RFC3339, li.CreatedAt)
	issue.UpdatedAt, _ = time.Parse(time.RFC3339, li.UpdatedAt)
	return issue
}

func (r *Repository) List(ctx context.Context, filter domain.ListFilter) ([]domain.Issue, error) {
	slog.DebugContext(ctx, "list issues", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "list")
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	parts := []string{fmt.Sprintf(`team: { key: { eq: "%s" } }`, r.team)}
	if filter.Status != "" {
		parts = append(parts, fmt.Sprintf(`state: { type: { eq: "%s" } }`, reverseStatus(filter.Status)))
	}
	if filter.Assignee != "" {
		parts = append(parts, fmt.Sprintf(`assignee: { name: { eq: "%s" } }`, filter.Assignee))
	}

	q := fmt.Sprintf(`{ issues(first: %d, filter: { %s }) { nodes { %s } } }`,
		limit, strings.Join(parts, ", "), issueFields)

	var result struct {
		Issues struct {
			Nodes []linearIssue `json:"nodes"`
		} `json:"issues"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Issue, len(result.Issues.Nodes))
	for i := range result.Issues.Nodes {
		out[i] = result.Issues.Nodes[i].toDomain()
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	slog.DebugContext(ctx, "get issue", logKeyBackend, BackendName, logKeyIssueKey, key, logKeyOperation, "get")
	num := extractNumber(key)
	q := fmt.Sprintf(`{ issues(filter: { team: { key: { eq: "%s" } }, number: { eq: %s } }) { nodes { %s } } }`,
		r.team, num, issueFields)

	var result struct {
		Issues struct {
			Nodes []linearIssue `json:"nodes"`
		} `json:"issues"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if len(result.Issues.Nodes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrIssueNotFound, key)
	}
	issue := result.Issues.Nodes[0].toDomain()
	return &issue, nil
}

func (r *Repository) Create(ctx context.Context, input domain.CreateInput) (*domain.Issue, error) {
	slog.InfoContext(ctx, "create issue", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "create", logKeyTitle, input.Title)
	parts := []string{fmt.Sprintf(`teamId: "%s"`, r.teamID)}
	parts = append(parts, fmt.Sprintf(`title: "%s"`, escape(input.Title)))
	if input.Description != "" {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(input.Description)))
	}
	if input.Priority != domain.PriorityNone {
		parts = append(parts, fmt.Sprintf(`priority: %d`, input.Priority))
	}
	if input.ParentID != "" {
		parts = append(parts, fmt.Sprintf(`parentId: "%s"`, input.ParentID))
	}
	if input.ProjectID != "" {
		parts = append(parts, fmt.Sprintf(`projectId: "%s"`, input.ProjectID))
	}
	if input.Assignee != "" {
		if userID, err := r.ResolveUser(ctx, input.Assignee); err == nil {
			parts = append(parts, fmt.Sprintf(`assigneeId: "%s"`, userID))
		}
	}

	q := fmt.Sprintf(`mutation { issueCreate(input: { %s }) { success issue { %s } } }`,
		strings.Join(parts, ", "), issueFields)

	var result struct {
		IssueCreate struct {
			Success bool        `json:"success"`
			Issue   linearIssue `json:"issue"`
		} `json:"issueCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.IssueCreate.Success {
		return nil, ErrCreateFailed
	}
	issue := result.IssueCreate.Issue.toDomain()
	return &issue, nil
}

func (r *Repository) Update(ctx context.Context, key string, input domain.UpdateInput) (*domain.Issue, error) {
	slog.InfoContext(ctx, "update issue", logKeyBackend, BackendName, logKeyIssueKey, key, logKeyOperation, "update")
	existing, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}

	var parts []string
	if input.Title != nil {
		parts = append(parts, fmt.Sprintf(`title: "%s"`, escape(*input.Title)))
	}
	if input.Description != nil {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(*input.Description)))
	}
	if input.Priority != nil {
		parts = append(parts, fmt.Sprintf(`priority: %d`, *input.Priority))
	}
	if input.Status != nil {
		stateID, err := r.resolveState(ctx, *input.Status)
		if err != nil {
			return nil, err
		}
		parts = append(parts, fmt.Sprintf(`stateId: "%s"`, stateID))
	}
	if len(parts) == 0 {
		return existing, nil
	}

	q := fmt.Sprintf(`mutation { issueUpdate(id: "%s", input: { %s }) { success issue { %s } } }`,
		existing.ID, strings.Join(parts, ", "), issueFields)

	var result struct {
		IssueUpdate struct {
			Success bool        `json:"success"`
			Issue   linearIssue `json:"issue"`
		} `json:"issueUpdate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	issue := result.IssueUpdate.Issue.toDomain()
	return &issue, nil
}

func (r *Repository) Search(ctx context.Context, query string, limit int) ([]domain.Issue, error) {
	slog.DebugContext(ctx, "search issues", logKeyBackend, BackendName, logKeyOperation, "search", logKeyQuery, query)
	if limit == 0 {
		limit = 20
	}
	q := fmt.Sprintf(`{ searchIssues(term: "%s", first: %d) { nodes { %s } } }`,
		escape(query), limit, issueFields)

	var result struct {
		SearchIssues struct {
			Nodes []linearIssue `json:"nodes"`
		} `json:"searchIssues"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Issue, len(result.SearchIssues.Nodes))
	for i := range result.SearchIssues.Nodes {
		out[i] = result.SearchIssues.Nodes[i].toDomain()
	}
	return out, nil
}

func (r *Repository) resolveState(ctx context.Context, status domain.Status) (string, error) {
	var result struct {
		Team struct {
			States struct {
				Nodes []struct {
					ID   string `json:"id"`
					Type string `json:"type"`
				} `json:"nodes"`
			} `json:"states"`
		} `json:"team"`
	}
	q := fmt.Sprintf(`{ team(id: "%s") { states { nodes { id type } } } }`, r.teamID)
	if err := r.gql(ctx, q, &result); err != nil {
		return "", err
	}
	target := reverseStatus(status)
	for _, s := range result.Team.States.Nodes {
		if s.Type == target {
			return s.ID, nil
		}
	}
	return "", fmt.Errorf("%w %q", ErrNoStateMatch, status)
}

func mapStatus(t string) domain.Status {
	switch t {
	case stateBacklog:
		return domain.StatusBacklog
	case stateUnstarted:
		return domain.StatusTodo
	case stateStarted:
		return domain.StatusInProgress
	case stateCompleted:
		return domain.StatusDone
	case stateCanceled:
		return domain.StatusCanceled
	default:
		return domain.StatusTodo
	}
}

func reverseStatus(s domain.Status) string {
	switch s {
	case domain.StatusBacklog:
		return stateBacklog
	case domain.StatusTodo:
		return stateUnstarted
	case domain.StatusInProgress, domain.StatusInReview:
		return stateStarted
	case domain.StatusDone:
		return stateCompleted
	case domain.StatusCanceled:
		return stateCanceled
	default:
		return stateUnstarted
	}
}

func extractNumber(key string) string {
	if i := strings.LastIndex(key, "-"); i >= 0 {
		return key[i+1:]
	}
	return key
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

// --- Document operations ---

type linearDocument struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	URL       string `json:"url"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Project   *struct {
		ID string `json:"id"`
	} `json:"project"`
}

const documentFields = `id title content url createdAt updatedAt project { id }`

func (ld *linearDocument) toDomain() domain.Document {
	doc := domain.Document{
		ID:      ld.ID,
		Title:   ld.Title,
		Content: ld.Content,
		URL:     ld.URL,
	}
	if ld.Project != nil {
		doc.ProjectID = ld.Project.ID
	}
	doc.CreatedAt, _ = time.Parse(time.RFC3339, ld.CreatedAt)
	doc.UpdatedAt, _ = time.Parse(time.RFC3339, ld.UpdatedAt)
	return doc
}

//nolint:dupl // list methods share patterns by design
func (r *Repository) ListDocuments(ctx context.Context, filter domain.DocumentListFilter) ([]domain.Document, error) {
	slog.DebugContext(ctx, "list documents", logKeyBackend, BackendName, logKeyOperation, "list_documents")
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	q := fmt.Sprintf(`{ documents(first: %d) { nodes { %s } } }`, limit, documentFields)

	var result struct {
		Documents struct {
			Nodes []linearDocument `json:"nodes"`
		} `json:"documents"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Document, len(result.Documents.Nodes))
	for i, ld := range result.Documents.Nodes {
		out[i] = ld.toDomain()
	}
	return out, nil
}

func (r *Repository) CreateDocument(ctx context.Context, input domain.DocumentCreateInput) (*domain.Document, error) {
	slog.InfoContext(ctx, "create document", logKeyBackend, BackendName, logKeyOperation, "create_document", logKeyTitle, input.Title)
	parts := []string{fmt.Sprintf(`title: "%s"`, escape(input.Title))}
	if input.Content != "" {
		parts = append(parts, fmt.Sprintf(`content: "%s"`, escape(input.Content)))
	}
	if input.ProjectID != "" {
		parts = append(parts, fmt.Sprintf(`projectId: "%s"`, input.ProjectID))
	}

	q := fmt.Sprintf(`mutation { documentCreate(input: { %s }) { success document { %s } } }`,
		strings.Join(parts, ", "), documentFields)

	var result struct {
		DocumentCreate struct {
			Success  bool           `json:"success"`
			Document linearDocument `json:"document"`
		} `json:"documentCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.DocumentCreate.Success {
		return nil, ErrDocumentCreate
	}
	doc := result.DocumentCreate.Document.toDomain()
	return &doc, nil
}

// --- Project operations ---

type linearProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	State       string `json:"state"`
	URL         string `json:"url"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

const projectFields = `id name description state url createdAt updatedAt`

func (lp *linearProject) toDomain() domain.Project {
	proj := domain.Project{
		ID:          lp.ID,
		Name:        lp.Name,
		Description: lp.Description,
		Status:      lp.State,
		URL:         lp.URL,
	}
	proj.CreatedAt, _ = time.Parse(time.RFC3339, lp.CreatedAt)
	proj.UpdatedAt, _ = time.Parse(time.RFC3339, lp.UpdatedAt)
	return proj
}

//nolint:dupl // list methods share patterns by design
func (r *Repository) ListProjects(ctx context.Context, filter domain.ProjectListFilter) ([]domain.Project, error) {
	slog.DebugContext(ctx, "list projects", logKeyBackend, BackendName, logKeyOperation, "list_projects")
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	q := fmt.Sprintf(`{ projects(first: %d) { nodes { %s } } }`, limit, projectFields)

	var result struct {
		Projects struct {
			Nodes []linearProject `json:"nodes"`
		} `json:"projects"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Project, len(result.Projects.Nodes))
	for i, lp := range result.Projects.Nodes {
		out[i] = lp.toDomain()
	}
	return out, nil
}

func (r *Repository) CreateProject(ctx context.Context, input domain.ProjectCreateInput) (*domain.Project, error) {
	slog.InfoContext(ctx, "create project", logKeyBackend, BackendName, logKeyOperation, "create_project", logKeyName, input.Name)
	teamIDs := input.TeamIDs
	if len(teamIDs) == 0 {
		teamIDs = []string{r.teamID}
	}
	teamIDsStr := make([]string, len(teamIDs))
	for i, id := range teamIDs {
		teamIDsStr[i] = fmt.Sprintf(`"%s"`, id)
	}

	parts := []string{
		fmt.Sprintf(`name: "%s"`, escape(input.Name)),
		fmt.Sprintf(`teamIds: [%s]`, strings.Join(teamIDsStr, ", ")),
	}
	if input.Description != "" {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(input.Description)))
	}

	q := fmt.Sprintf(`mutation { projectCreate(input: { %s }) { success project { %s } } }`,
		strings.Join(parts, ", "), projectFields)

	var result struct {
		ProjectCreate struct {
			Success bool          `json:"success"`
			Project linearProject `json:"project"`
		} `json:"projectCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.ProjectCreate.Success {
		return nil, ErrProjectCreate
	}
	proj := result.ProjectCreate.Project.toDomain()
	return &proj, nil
}

func (r *Repository) UpdateProject(ctx context.Context, id string, input domain.ProjectUpdateInput) (*domain.Project, error) {
	slog.InfoContext(ctx, "update project", logKeyBackend, BackendName, logKeyOperation, "update_project", logKeyID, id)

	var parts []string
	if input.Name != nil {
		parts = append(parts, fmt.Sprintf(`name: "%s"`, escape(*input.Name)))
	}
	if input.Description != nil {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(*input.Description)))
	}
	if len(parts) == 0 {
		// Nothing to update — just fetch and return
		return r.getProject(ctx, id)
	}

	q := fmt.Sprintf(`mutation { projectUpdate(id: "%s", input: { %s }) { success project { %s } } }`,
		id, strings.Join(parts, ", "), projectFields)

	var result struct {
		ProjectUpdate struct {
			Success bool          `json:"success"`
			Project linearProject `json:"project"`
		} `json:"projectUpdate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.ProjectUpdate.Success {
		return nil, ErrProjectUpdate
	}
	proj := result.ProjectUpdate.Project.toDomain()
	return &proj, nil
}

func (r *Repository) getProject(ctx context.Context, id string) (*domain.Project, error) {
	q := fmt.Sprintf(`{ node(id: "%s") { ... on Project { %s } } }`, id, projectFields)
	var result struct {
		Node linearProject `json:"node"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	proj := result.Node.toDomain()
	return &proj, nil
}

// --- Initiative operations ---

type linearInitiative struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Status      string `json:"status"`
	URL         string `json:"url"`
}

const initiativeFields = `id name description status`

func (li *linearInitiative) toDomain() domain.Initiative {
	return domain.Initiative{
		ID:          li.ID,
		Name:        li.Name,
		Description: li.Description,
		Status:      li.Status,
	}
}

//nolint:dupl // list methods share patterns by design
func (r *Repository) ListInitiatives(ctx context.Context, filter domain.InitiativeListFilter) ([]domain.Initiative, error) {
	slog.DebugContext(ctx, "list initiatives", logKeyBackend, BackendName, logKeyOperation, "list_initiatives")
	limit := filter.Limit
	if limit == 0 {
		limit = defaultLimit
	}
	q := fmt.Sprintf(`{ initiatives(first: %d) { nodes { %s } } }`, limit, initiativeFields)

	var result struct {
		Initiatives struct {
			Nodes []linearInitiative `json:"nodes"`
		} `json:"initiatives"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Initiative, len(result.Initiatives.Nodes))
	for i, li := range result.Initiatives.Nodes {
		out[i] = li.toDomain()
	}
	return out, nil
}

func (r *Repository) CreateInitiative(ctx context.Context, input domain.InitiativeCreateInput) (*domain.Initiative, error) {
	slog.InfoContext(ctx, "create initiative", logKeyBackend, BackendName, logKeyOperation, "create_initiative", logKeyName, input.Name)
	parts := []string{fmt.Sprintf(`name: "%s"`, escape(input.Name))}
	if input.Description != "" {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(input.Description)))
	}

	q := fmt.Sprintf(`mutation { initiativeCreate(input: { %s }) { success initiative { %s } } }`,
		strings.Join(parts, ", "), initiativeFields)

	var result struct {
		InitiativeCreate struct {
			Success    bool             `json:"success"`
			Initiative linearInitiative `json:"initiative"`
		} `json:"initiativeCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.InitiativeCreate.Success {
		return nil, ErrInitiativeCreate
	}
	init := result.InitiativeCreate.Initiative.toDomain()
	return &init, nil
}

// --- Label operations ---

func (r *Repository) ListLabels(ctx context.Context) ([]domain.Label, error) {
	slog.DebugContext(ctx, "list labels", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "list_labels")
	q := fmt.Sprintf(`{ issueLabels(filter: { team: { id: { eq: "%s" } } }) { nodes { id name color } } }`, r.teamID)

	var result struct {
		IssueLabels struct {
			Nodes []struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"nodes"`
		} `json:"issueLabels"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	out := make([]domain.Label, len(result.IssueLabels.Nodes))
	for i, l := range result.IssueLabels.Nodes {
		out[i] = domain.Label{ID: l.ID, Name: l.Name, Color: l.Color}
	}
	return out, nil
}

func (r *Repository) CreateLabel(ctx context.Context, input domain.LabelCreateInput) (*domain.Label, error) {
	slog.InfoContext(ctx, "create label", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "create_label", logKeyName, input.Name)
	parts := []string{
		fmt.Sprintf(`name: "%s"`, escape(input.Name)),
		fmt.Sprintf(`teamId: "%s"`, r.teamID),
	}
	if input.Color != "" {
		parts = append(parts, fmt.Sprintf(`color: "%s"`, escape(input.Color)))
	}

	q := fmt.Sprintf(`mutation { issueLabelCreate(input: { %s }) { success issueLabel { id name color } } }`,
		strings.Join(parts, ", "))

	var result struct {
		IssueLabelCreate struct {
			Success    bool `json:"success"`
			IssueLabel struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Color string `json:"color"`
			} `json:"issueLabel"`
		} `json:"issueLabelCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.IssueLabelCreate.Success {
		return nil, ErrLabelCreate
	}
	l := result.IssueLabelCreate.IssueLabel
	return &domain.Label{ID: l.ID, Name: l.Name, Color: l.Color}, nil
}

// --- Bulk operations ---

func (r *Repository) BulkCreateIssues(ctx context.Context, inputs []domain.CreateInput) ([]domain.Issue, error) {
	slog.InfoContext(ctx, "bulk create issues", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "bulk_create", logKeyCount, len(inputs))
	if len(inputs) == 0 {
		return nil, nil
	}

	issueInputs := make([]string, 0, len(inputs))
	for i := range inputs {
		parts := []string{
			fmt.Sprintf(`teamId: "%s"`, r.teamID),
			fmt.Sprintf(`title: "%s"`, escape(inputs[i].Title)),
		}
		if inputs[i].Description != "" {
			parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(inputs[i].Description)))
		}
		if inputs[i].Priority != domain.PriorityNone {
			parts = append(parts, fmt.Sprintf(`priority: %d`, inputs[i].Priority))
		}
		if inputs[i].ParentID != "" {
			parts = append(parts, fmt.Sprintf(`parentId: "%s"`, inputs[i].ParentID))
		}
		if inputs[i].ProjectID != "" {
			parts = append(parts, fmt.Sprintf(`projectId: "%s"`, inputs[i].ProjectID))
		}
		if inputs[i].Assignee != "" {
			userID, err := r.ResolveUser(ctx, inputs[i].Assignee)
			if err == nil {
				parts = append(parts, fmt.Sprintf(`assigneeId: "%s"`, userID))
			}
		}
		issueInputs = append(issueInputs, fmt.Sprintf(`{ %s }`, strings.Join(parts, ", ")))
	}

	q := fmt.Sprintf(`mutation { issueBatchCreate(issues: [%s]) { success issues { %s } } }`,
		strings.Join(issueInputs, ", "), issueFields)

	var result struct {
		IssueBatchCreate struct {
			Success bool          `json:"success"`
			Issues  []linearIssue `json:"issues"`
		} `json:"issueBatchCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.IssueBatchCreate.Success {
		return nil, ErrBulkCreate
	}
	out := make([]domain.Issue, len(result.IssueBatchCreate.Issues))
	for i := range result.IssueBatchCreate.Issues {
		out[i] = result.IssueBatchCreate.Issues[i].toDomain()
	}
	return out, nil
}

// --- Sub-issue / children operations ---

func (r *Repository) ListChildren(ctx context.Context, key string) ([]domain.Issue, error) {
	slog.DebugContext(ctx, "list children", logKeyBackend, BackendName, logKeyIssueKey, key, logKeyOperation, "list_children")
	num := extractNumber(key)
	q := fmt.Sprintf(`{ issues(filter: { team: { key: { eq: "%s" } }, number: { eq: %s } }) { nodes { children { nodes { %s } } } } }`,
		r.team, num, issueFields)

	var result struct {
		Issues struct {
			Nodes []struct {
				Children struct {
					Nodes []linearIssue `json:"nodes"`
				} `json:"children"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if len(result.Issues.Nodes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrIssueNotFound, key)
	}
	children := result.Issues.Nodes[0].Children.Nodes
	out := make([]domain.Issue, len(children))
	for i := range children {
		out[i] = children[i].toDomain()
	}
	return out, nil
}

// --- User resolution ---

func (r *Repository) ResolveUser(ctx context.Context, name string) (string, error) {
	q := fmt.Sprintf(`{ users(filter: { name: { containsIgnoreCase: "%s" } }) { nodes { id name } } }`, escape(name))

	var result struct {
		Users struct {
			Nodes []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"nodes"`
		} `json:"users"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return "", err
	}
	if len(result.Users.Nodes) == 0 {
		return "", fmt.Errorf("%w: %s", ErrUserNotFound, name)
	}
	return result.Users.Nodes[0].ID, nil
}

// --- Comment operations ---

type linearComment struct {
	ID        string `json:"id"`
	Body      string `json:"body"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	User      *struct {
		Name string `json:"name"`
	} `json:"user"`
}

func (lc *linearComment) toDomain() domain.Comment {
	c := domain.Comment{
		ID:   lc.ID,
		Body: lc.Body,
	}
	if lc.User != nil {
		c.Author = lc.User.Name
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339, lc.CreatedAt)
	c.UpdatedAt, _ = time.Parse(time.RFC3339, lc.UpdatedAt)
	return c
}

func (r *Repository) ListComments(ctx context.Context, key string) ([]domain.Comment, error) {
	num := extractNumber(key)
	q := fmt.Sprintf(`{ issues(filter: { team: { key: { eq: "%s" } }, number: { eq: %s } }) { nodes { comments { nodes { id body createdAt updatedAt user { name } } } } } }`,
		r.team, num)
	var result struct {
		Issues struct {
			Nodes []struct {
				Comments struct {
					Nodes []linearComment `json:"nodes"`
				} `json:"comments"`
			} `json:"nodes"`
		} `json:"issues"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if len(result.Issues.Nodes) == 0 {
		return nil, fmt.Errorf("%w: %s", ErrIssueNotFound, key)
	}
	raw := result.Issues.Nodes[0].Comments.Nodes
	comments := make([]domain.Comment, len(raw))
	for i := range raw {
		comments[i] = raw[i].toDomain()
	}
	return comments, nil
}

func (r *Repository) AddComment(ctx context.Context, key string, input domain.CommentCreateInput) (*domain.Comment, error) {
	existing, err := r.Get(ctx, key)
	if err != nil {
		return nil, err
	}
	escaped := escapeGraphQL(input.Body)
	q := fmt.Sprintf(`mutation { commentCreate(input: { issueId: "%s", body: "%s" }) { success comment { id body createdAt updatedAt user { name } } } }`,
		existing.ID, escaped)
	var result struct {
		CommentCreate struct {
			Success bool          `json:"success"`
			Comment linearComment `json:"comment"`
		} `json:"commentCreate"`
	}
	if err := r.gql(ctx, q, &result); err != nil {
		return nil, err
	}
	if !result.CommentCreate.Success {
		return nil, fmt.Errorf("%w on %s", ErrCommentCreateFailed, key)
	}
	c := result.CommentCreate.Comment.toDomain()
	return &c, nil
}

func escapeGraphQL(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}
