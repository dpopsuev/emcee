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

	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
)

// Sentinel errors.
var (
	ErrIssueNotFound = errors.New("issue not found")
	ErrCreateFailed  = errors.New("issue creation failed")
	ErrTeamNotFound  = errors.New("team not found")
)

// Compile-time interface compliance check.
var _ driven.IssueRepository = (*Repository)(nil)

// Repository implements driven.IssueRepository for Linear.
type Repository struct {
	apiKey string
	teamID string
	team   string
	client *http.Client
}

// New creates a Linear repository. It resolves the team key to an ID on init.
func New(apiKey, teamKey string) (*Repository, error) {
	r := &Repository{
		apiKey: apiKey,
		team:   teamKey,
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
	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
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
		return fmt.Errorf("graphql: %s", gqlResp.Errors[0].Message)
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
	slog.Debug("list issues", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "list")
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
	for i, li := range result.Issues.Nodes {
		out[i] = li.toDomain()
	}
	return out, nil
}

func (r *Repository) Get(ctx context.Context, key string) (*domain.Issue, error) {
	slog.Debug("get issue", logKeyBackend, BackendName, logKeyIssueKey, key, logKeyOperation, "get")
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
	slog.Info("create issue", logKeyBackend, BackendName, logKeyTeam, r.team, logKeyOperation, "create", "title", input.Title)
	parts := []string{fmt.Sprintf(`teamId: "%s"`, r.teamID)}
	parts = append(parts, fmt.Sprintf(`title: "%s"`, escape(input.Title)))
	if input.Description != "" {
		parts = append(parts, fmt.Sprintf(`description: "%s"`, escape(input.Description)))
	}
	if input.Priority != domain.PriorityNone {
		parts = append(parts, fmt.Sprintf(`priority: %d`, input.Priority))
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
	slog.Info("update issue", logKeyBackend, BackendName, logKeyIssueKey, key, logKeyOperation, "update")
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
	slog.Debug("search issues", logKeyBackend, BackendName, logKeyOperation, "search", "query", query)
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
	for i, li := range result.SearchIssues.Nodes {
		out[i] = li.toDomain()
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
	return "", fmt.Errorf("no state matching %q", status)
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
