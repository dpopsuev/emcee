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
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/dpopsuev/emcee/internal/domain"
	infra "github.com/dpopsuev/emcee/internal/infrastructure"
	"github.com/dpopsuev/emcee/internal/repository"
)

const (
	BackendName    = "reportportal"
	defaultTimeout = 30 * time.Second
	defaultLimit   = 50
	statusFailed   = "FAILED"
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
	_ repository.IssueRepository  = (*Repository)(nil)
	_ repository.LaunchRepository = (*Repository)(nil)
)

// Repository implements repository.LaunchRepository for Report Portal.
type Repository struct {
	name    string
	baseURL string // e.g. https://reportportal.example.com
	project string // RP project name
	token   string // API key
	client  *http.Client
}

// New creates a Report Portal repository.
func New(name, baseURL, project, token string) (*Repository, error) {
	baseURL = strings.TrimRight(baseURL, "/")
	return &Repository{
		name:    name,
		baseURL: baseURL,
		project: project,
		token:   token,
		client:  &http.Client{Timeout: defaultTimeout},
	}, nil
}

func (r *Repository) Name() string { return r.name }

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
		sanitized := infra.SanitizeError(string(respBody))
		infra.LogAPIError(ctx, BackendName, method, path, resp.StatusCode, sanitized)
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
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Status      string          `json:"status"`
	Description string          `json:"description"`
	Owner       string          `json:"owner"`
	StartTime   json.RawMessage `json:"startTime"`
	EndTime     json.RawMessage `json:"endTime"`
	Attributes  []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"attributes"`
	Statistics struct {
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
	launch.StartTime = parseRPTimestamp(l.StartTime)
	launch.EndTime = parseRPTimestamp(l.EndTime)
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
	if len(l.Attributes) > 0 {
		launch.Attributes = make([]domain.LaunchAttribute, 0, len(l.Attributes))
		for _, a := range l.Attributes {
			launch.Attributes = append(launch.Attributes, domain.LaunchAttribute{Key: a.Key, Value: a.Value})
		}
	}
	return launch
}

// parseRPTimestamp handles both int64 (epoch millis) and string (ISO 8601) formats.
func parseRPTimestamp(raw json.RawMessage) time.Time {
	if len(raw) == 0 || string(raw) == "null" {
		return time.Time{}
	}
	// Try int64 first (epoch millis)
	var ms int64
	if err := json.Unmarshal(raw, &ms); err == nil && ms > 0 {
		return time.UnixMilli(ms)
	}
	// Try string (ISO 8601)
	var s string
	if err := json.Unmarshal(raw, &s); err == nil && s != "" {
		for _, layout := range []string{
			time.RFC3339,
			"2006-01-02T15:04:05.000Z",
			"2006-01-02T15:04:05Z",
			time.RFC3339Nano,
		} {
			if t, err := time.Parse(layout, s); err == nil {
				return t
			}
		}
	}
	return time.Time{}
}

func (r *Repository) ListLaunches(ctx context.Context, filter domain.LaunchFilter) ([]domain.Launch, error) {
	infra.LogOp(ctx, BackendName, "list_launches")
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	page := filter.Page
	if page < 0 {
		page = 0
	}
	path := fmt.Sprintf("/launch?page.size=%d&page.number=%d&page.sort=startTime,desc", limit, page)
	if filter.Name != "" {
		path += "&filter.cnt.name=" + url.QueryEscape(filter.Name)
	}
	if filter.Status != "" {
		path += "&filter.eq.status=" + strings.ToUpper(filter.Status)
	}
	if !filter.StartAfter.IsZero() || !filter.StartBefore.IsZero() {
		after := int64(0)
		if !filter.StartAfter.IsZero() {
			after = filter.StartAfter.UnixMilli()
		}
		before := int64(0)
		if !filter.StartBefore.IsZero() {
			before = filter.StartBefore.UnixMilli()
		}
		path += fmt.Sprintf("&filter.btw.startTime=%d,%d", after, before)
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
	infra.LogOp(ctx, BackendName, "get_launch", slog.String(infra.LogKeyID, id))
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
	ParentID int    `json:"parent"`
	LaunchID int    `json:"launchId"`
	Issue    *struct {
		IssueType            string `json:"issueType"`
		Comment              string `json:"comment"`
		ExternalSystemIssues []struct {
			TicketID   string `json:"ticketId"`
			BtsURL     string `json:"btsUrl"`
			BtsProject string `json:"btsProject"`
			URL        string `json:"url"`
		} `json:"externalSystemIssues"`
	} `json:"issue"`
}

func (ti *rpTestItem) toDomain(baseURL, project string) domain.TestItem {
	item := domain.TestItem{
		ID:       strconv.Itoa(ti.ID),
		Name:     ti.Name,
		Status:   ti.Status,
		Type:     ti.Type,
		ParentID: strconv.Itoa(ti.ParentID),
		LaunchID: strconv.Itoa(ti.LaunchID),
		URL:      fmt.Sprintf("%s/ui/#%s/launches/all/%d", baseURL, project, ti.LaunchID),
	}
	if ti.Issue != nil {
		item.IssueType = ti.Issue.IssueType
		item.Comment = ti.Issue.Comment
		for _, esi := range ti.Issue.ExternalSystemIssues {
			item.ExternalSystemIssues = append(item.ExternalSystemIssues, domain.ExternalSystemIssue{
				TicketID:   esi.TicketID,
				BtsURL:     esi.BtsURL,
				BtsProject: esi.BtsProject,
				URL:        esi.URL,
			})
		}
	}
	return item
}

func (r *Repository) ListTestItems(ctx context.Context, launchID string, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	infra.LogOp(ctx, BackendName, "list_test_items", slog.String(infra.LogKeyID, launchID))
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	page := filter.Page
	if page < 0 {
		page = 0
	}
	params := url.Values{
		"filter.eq.launchId":    {launchID},
		"filter.eq.hasChildren": {"false"},
		"isLatest":              {"false"},
		"launchesLimit":         {"0"},
		"page.size":             {strconv.Itoa(limit)},
		"page.number":           {strconv.Itoa(page)},
	}
	if filter.Status != "" {
		params.Set("filter.eq.status", strings.ToUpper(filter.Status))
	}
	if filter.IssueType != "" {
		params.Set("filter.eq.issueType", filter.IssueType)
	}
	if filter.Name != "" {
		params.Set("filter.cnt.name", filter.Name)
	}

	var result struct {
		Content []rpTestItem `json:"content"`
	}
	if err := r.api(ctx, "GET", "/item?"+params.Encode(), nil, &result); err != nil {
		return nil, err
	}

	items := make([]domain.TestItem, 0, len(result.Content))
	for i := range result.Content {
		item := result.Content[i].toDomain(r.baseURL, r.project)
		if filter.IncludeLogs && item.Status == statusFailed {
			item.FailureMessage = r.fetchErrorLogs(ctx, item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

// SearchTestItems searches test items across one or more launches.
// LaunchIDs must be populated by the application layer before calling this method —
// the RP API requires at least one launch ID (filter.in.launchId) or it returns 400.
func (r *Repository) SearchTestItems(ctx context.Context, filter domain.TestItemFilter) ([]domain.TestItem, error) {
	infra.LogOp(ctx, BackendName, "search_test_items")
	if len(filter.LaunchIDs) == 0 {
		return nil, fmt.Errorf("%w: SearchTestItems requires at least one launch ID; "+
			"set Since/Before/LaunchName so the application layer can resolve them", ErrAPIError)
	}
	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	page := filter.Page
	if page < 0 {
		page = 0
	}
	params := url.Values{
		"filter.eq.hasChildren": {"false"},
		"isLatest":              {"false"},
		"launchesLimit":         {"0"},
		"page.size":             {strconv.Itoa(limit)},
		"page.number":           {strconv.Itoa(page)},
	}
	if filter.Status != "" {
		params.Set("filter.eq.status", strings.ToUpper(filter.Status))
	}
	if filter.IssueType != "" {
		params.Set("filter.eq.issueType", filter.IssueType)
	}
	if filter.Name != "" {
		params.Set("filter.cnt.name", filter.Name)
	}
	// RP requires literal commas for filter.in.* — url.Values.Encode() would
	// produce %2C which RP rejects. Build the launchId param manually.
	path := "/item?filter.in.launchId=" + strings.Join(filter.LaunchIDs, ",") + "&" + params.Encode()

	var result struct {
		Content []rpTestItem `json:"content"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}

	items := make([]domain.TestItem, 0, len(result.Content))
	for i := range result.Content {
		item := result.Content[i].toDomain(r.baseURL, r.project)
		if filter.IncludeLogs && item.Status == statusFailed {
			item.FailureMessage = r.fetchErrorLogs(ctx, item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

func (r *Repository) GetTestItem(ctx context.Context, id string) (*domain.TestItem, error) {
	infra.LogOp(ctx, BackendName, "get_test_item", slog.String(infra.LogKeyID, id))
	var raw rpTestItem
	if err := r.api(ctx, "GET", "/item/"+id, nil, &raw); err != nil {
		if errors.Is(err, ErrLaunchNotFound) {
			return nil, ErrTestItemNotFound
		}
		return nil, err
	}
	item := raw.toDomain(r.baseURL, r.project)
	if item.Status == statusFailed {
		item.FailureMessage = r.fetchErrorLogs(ctx, id)
	}
	return &item, nil
}

type rpLogEntry struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
	Level   string `json:"level"`
}

func (r *Repository) fetchErrorLogs(ctx context.Context, itemID string) string {
	path := fmt.Sprintf("/log?filter.eq.item=%s&filter.in.level=ERROR&filter.in.level=TRACE&page.size=10", itemID)
	var result struct {
		Content []rpLogEntry `json:"content"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return ""
	}
	if len(result.Content) == 0 {
		return ""
	}
	var sb strings.Builder
	for i := range result.Content {
		if i > 0 {
			sb.WriteString("\n")
		}
		sb.WriteString(result.Content[i].Message)
	}
	return sb.String()
}

func (r *Repository) GetTestItems(ctx context.Context, ids []string) ([]domain.TestItem, error) {
	infra.LogOp(ctx, BackendName, "get_test_items", slog.Int(infra.LogKeyCount, len(ids)))
	path := fmt.Sprintf("/item?filter.in.id=%s&page.size=%d", strings.Join(ids, ","), len(ids))
	var result struct {
		Content []rpTestItem `json:"content"`
	}
	if err := r.api(ctx, "GET", path, nil, &result); err != nil {
		return nil, err
	}
	items := make([]domain.TestItem, 0, len(result.Content))
	for i := range result.Content {
		item := result.Content[i].toDomain(r.baseURL, r.project)
		if item.Status == statusFailed {
			item.FailureMessage = r.fetchErrorLogs(ctx, item.ID)
		}
		items = append(items, item)
	}
	return items, nil
}

// --- Defect update (bulk endpoint per NED-5: always use PUT /item, not PUT /item/{id}/update) ---

func (r *Repository) UpdateDefects(ctx context.Context, updates []domain.DefectUpdate) error {
	infra.LogWrite(ctx, BackendName, "update_defects", slog.Int(infra.LogKeyCount, len(updates)))
	type btsIssue struct {
		TicketID   string `json:"ticketId"`
		BtsURL     string `json:"btsUrl"`
		BtsProject string `json:"btsProject"`
		URL        string `json:"url,omitempty"`
	}
	type issueUpdate struct {
		TestItemID int `json:"testItemId"`
		Issue      struct {
			IssueType            string     `json:"issueType"`
			Comment              string     `json:"comment,omitempty"`
			ExternalSystemIssues []btsIssue `json:"externalSystemIssues,omitempty"`
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
		for _, esi := range u.ExternalSystemIssues {
			iu.Issue.ExternalSystemIssues = append(iu.Issue.ExternalSystemIssues, btsIssue{
				TicketID:   esi.TicketID,
				BtsURL:     esi.BtsURL,
				BtsProject: esi.BtsProject,
				URL:        esi.URL,
			})
		}
		issues = append(issues, iu)
	}

	body := map[string]any{"issues": issues}
	return r.api(ctx, "PUT", "/item", body, nil)
}

// --- Dashboard operations ---

type rpDashboard struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (d *rpDashboard) toDomain() domain.Dashboard {
	return domain.Dashboard{
		ID:          strconv.Itoa(d.ID),
		Name:        d.Name,
		Description: d.Description,
	}
}

func (r *Repository) ListDashboards(ctx context.Context) ([]domain.Dashboard, error) {
	infra.LogOp(ctx, BackendName, "list_dashboards")
	var result struct {
		Content []rpDashboard `json:"content"`
	}
	if err := r.api(ctx, "GET", "/dashboard", nil, &result); err != nil {
		return nil, err
	}
	dashboards := make([]domain.Dashboard, 0, len(result.Content))
	for i := range result.Content {
		dashboards = append(dashboards, result.Content[i].toDomain())
	}
	return dashboards, nil
}

func (r *Repository) GetDashboard(ctx context.Context, id string) (*domain.Dashboard, error) {
	infra.LogOp(ctx, BackendName, "get_dashboard", slog.String(infra.LogKeyID, id))
	var raw rpDashboard
	if err := r.api(ctx, "GET", "/dashboard/"+id, nil, &raw); err != nil {
		return nil, err
	}
	d := raw.toDomain()
	return &d, nil
}

func (r *Repository) CreateDashboard(ctx context.Context, input domain.DashboardCreateInput) (*domain.Dashboard, error) {
	infra.LogWrite(ctx, BackendName, "create_dashboard", slog.String(infra.LogKeyName, input.Name))
	body := map[string]string{"name": input.Name, "description": input.Description}
	var result struct {
		ID int `json:"id"`
	}
	if err := r.api(ctx, "POST", "/dashboard", body, &result); err != nil {
		return nil, err
	}
	return &domain.Dashboard{ID: strconv.Itoa(result.ID), Name: input.Name, Description: input.Description}, nil
}

func (r *Repository) AddWidget(ctx context.Context, dashboardID string, input domain.WidgetAddInput) (*domain.Widget, error) {
	infra.LogWrite(ctx, BackendName, "add_widget", slog.String(infra.LogKeyID, dashboardID))
	body := map[string]any{
		"name":       input.Name,
		"widgetType": input.Type,
		"widgetSize": map[string]int{"width": input.Width, "height": input.Height},
	}
	var result struct {
		ID int `json:"id"`
	}
	path := fmt.Sprintf("/dashboard/%s/widget", dashboardID)
	if err := r.api(ctx, "POST", path, body, &result); err != nil {
		return nil, err
	}
	return &domain.Widget{ID: strconv.Itoa(result.ID), Name: input.Name, Type: input.Type}, nil
}
