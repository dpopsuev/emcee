// Package reportportal implements the driven (outbound) adapter for Report Portal's REST API v1.
package reportportal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

const (
	BackendName    = "reportportal"
	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
)

var (
	ErrLaunchNotFound   = errors.New("launch not found")
	ErrTestItemNotFound = errors.New("test item not found")
	ErrAPIError         = errors.New("reportportal API error")
)

// ErrNotIssueBackend indicates RP does not support traditional issue operations.
var ErrNotIssueBackend = errors.New("reportportal is not an issue backend — use launches/test_items actions")

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository  = (*Repository)(nil)
	_ driven.LaunchRepository = (*Repository)(nil)
)

// Repository implements driven.LaunchRepository for Report Portal.
type Repository struct {
	baseURL string // e.g. https://reportportal.example.com
	project string // RP project name
	token   string // API key
	client  *http.Client
}

// New creates a Report Portal repository.
func New(baseURL, project, token string) (*Repository, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Repository{
		baseURL: baseURL,
		project: project,
		token:   token,
		client:  &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (r *Repository) Name() string { return BackendName }

// --- IssueRepository stub (RP is not an issue backend, but needs to satisfy the registry) ---

func (r *Repository) List(_ context.Context, _ domain.ListFilter) ([]domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

func (r *Repository) Get(_ context.Context, _ string) (*domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

func (r *Repository) Create(_ context.Context, _ domain.CreateInput) (*domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

func (r *Repository) Update(_ context.Context, _ string, _ domain.UpdateInput) (*domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

func (r *Repository) Search(_ context.Context, _ string, _ int) ([]domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

func (r *Repository) ListChildren(_ context.Context, _ string) ([]domain.Issue, error) {
	return nil, ErrNotIssueBackend
}

// api makes an authenticated request to the Report Portal REST API.
func (r *Repository) api(ctx context.Context, method, path string, body, result any) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	fullURL := fmt.Sprintf("%s/api/v1/%s%s", r.baseURL, r.project, path)
	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+r.token)
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
		return ErrLaunchNotFound
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

// --- Launch operations ---

type rpLaunch struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Owner       string `json:"owner"`
	StartTime   int64  `json:"startTime"`
	EndTime     int64  `json:"endTime"`
	Statistics  struct {
		Executions struct {
			Total   int `json:"total"`
			Passed  int `json:"passed"`
			Failed  int `json:"failed"`
			Skipped int `json:"skipped"`
		} `json:"executions"`
		Defects map[string]map[string]int `json:"defects"`
	} `json:"statistics"`
}

func (l *rpLaunch) toDomain(baseURL, project string) domain.Launch {
	launch := domain.Launch{
		ID:          strconv.Itoa(l.ID),
		Name:        l.Name,
		Status:      l.Status,
		Description: l.Description,
		Owner:       l.Owner,
		URL:         fmt.Sprintf("%s/ui/#%s/launches/all/%d", baseURL, project, l.ID),
	}
	if l.StartTime > 0 {
		launch.StartTime = time.UnixMilli(l.StartTime)
	}
	if l.EndTime > 0 {
		launch.EndTime = time.UnixMilli(l.EndTime)
	}
	launch.Statistics = domain.LaunchStatistics{
		Total:   l.Statistics.Executions.Total,
		Passed:  l.Statistics.Executions.Passed,
		Failed:  l.Statistics.Executions.Failed,
		Skipped: l.Statistics.Executions.Skipped,
		Defects: make(map[string]int),
	}
	for category, counts := range l.Statistics.Defects {
		if total, ok := counts["total"]; ok {
			launch.Statistics.Defects[category] = total
		}
	}
	return launch
}

func (r *Repository) ListLaunches(ctx context.Context, filter domain.LaunchFilter) ([]domain.Launch, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_launches")
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	path := fmt.Sprintf("/launch?page.size=%d&page.sort=startTime,desc", limit)
	if filter.Name != "" {
		path += "&filter.cnt.name=" + filter.Name
	}
	if filter.Status != "" {
		path += "&filter.eq.status=" + strings.ToUpper(filter.Status)
	}

	var result struct {
		Content []rpLaunch `json:"content"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	launches := make([]domain.Launch, 0, len(result.Content))
	for i := range result.Content {
		launches = append(launches, result.Content[i].toDomain(r.baseURL, r.project))
	}
	return launches, nil
}

func (r *Repository) GetLaunch(ctx context.Context, id string) (*domain.Launch, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_launch", slog.String(adapterdriven.LogKeyID, id))
	var raw rpLaunch
	if err := r.api(ctx, "GET", "/launch/"+id, nil, &raw); err != nil {
		return nil, err
	}
	launch := raw.toDomain(r.baseURL, r.project)
	return &launch, nil
}

// --- Test item operations ---

type rpTestItem struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Type     string `json:"type"`
	LaunchID int    `json:"launchId"`
	Issue    *struct {
		IssueType string `json:"issueType"`
		Comment   string `json:"comment"`
	} `json:"issue"`
}

func (ti *rpTestItem) toDomain(baseURL, project string) domain.TestItem {
	item := domain.TestItem{
		ID:       strconv.Itoa(ti.ID),
		Name:     ti.Name,
		Status:   ti.Status,
		Type:     ti.Type,
		LaunchID: strconv.Itoa(ti.LaunchID),
		URL:      fmt.Sprintf("%s/ui/#%s/launches/all/%d", baseURL, project, ti.LaunchID),
	}
	if ti.Issue != nil {
		item.IssueType = ti.Issue.IssueType
		item.Comment = ti.Issue.Comment
	}
	return item
}

func (r *Repository) ListTestItems(ctx context.Context, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_test_items", slog.String(adapterdriven.LogKeyID, launchID))
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	path := fmt.Sprintf("/item?filter.eq.launchId=%s&filter.in.type=STEP&isLatest=false&launchesLimit=0&page.size=%d",
		launchID, limit)
	if filter.Status != "" {
		path += "&filter.eq.status=" + strings.ToUpper(filter.Status)
	}

	var result struct {
		Content []rpTestItem `json:"content"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	items := make([]domain.TestItem, 0, len(result.Content))
	for i := range result.Content {
		items = append(items, result.Content[i].toDomain(r.baseURL, r.project))
	}
	return items, nil
}

func (r *Repository) GetTestItem(ctx context.Context, id string) (*domain.TestItem, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_test_item", slog.String(adapterdriven.LogKeyID, id))
	var raw rpTestItem
	if err := r.api(ctx, "GET", "/item/"+id, nil, &raw); err != nil {
		if errors.Is(err, ErrLaunchNotFound) {
			return nil, ErrTestItemNotFound
		}
		return nil, err
	}
	item := raw.toDomain(r.baseURL, r.project)
	return &item, nil
}

// --- Defect update (bulk endpoint per NED-5: always use PUT /item, not PUT /item/{id}/update) ---

func (r *Repository) UpdateDefects(ctx context.Context, updates []domain.DefectUpdate) error {
	adapterdriven.LogWrite(ctx, BackendName, "update_defects", slog.Int(adapterdriven.LogKeyCount, len(updates)))
	type issueUpdate struct {
		TestItemID int `json:"testItemId"`
		Issue      struct {
			IssueType string `json:"issueType"`
			Comment   string `json:"comment,omitempty"`
		} `json:"issue"`
	}

	issues := make([]issueUpdate, 0, len(updates))
	for _, u := range updates {
		id, err := strconv.Atoi(u.TestItemID)
		if err != nil {
			return fmt.Errorf("invalid test item ID %q: %w", u.TestItemID, err)
		}
		iu := issueUpdate{TestItemID: id}
		iu.Issue.IssueType = u.IssueType
		iu.Issue.Comment = u.Comment
		issues = append(issues, iu)
	}

	body := map[string]any{"issues": issues}
	return r.api(ctx, "PUT", "/item", body, nil)
}
