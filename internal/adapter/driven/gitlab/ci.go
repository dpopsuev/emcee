package gitlab

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.CIRepository = (*Repository)(nil)

// encodedProject returns the project ID safe for URL path segments.
func (r *Repository) encodedProject() string {
	return url.PathEscape(r.projectID)
}

// ListPipelines lists CI pipelines.
func (r *Repository) ListPipelines(ctx context.Context, filter domain.CIPipelineFilter) ([]domain.CIPipeline, error) {
	adapterdriven.LogOp(ctx, BackendName, "ListPipelines")
	start := time.Now()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	path := fmt.Sprintf("/api/v4/projects/%s/pipelines?per_page=%d", r.encodedProject(), limit)
	if filter.Status != "" {
		path += "&status=" + filter.Status
	}
	if filter.Ref != "" {
		path += "&ref=" + filter.Ref
	}

	var resp []glPipeline
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "ListPipelines", err)
		return nil, err
	}
	pipelines := make([]domain.CIPipeline, len(resp))
	for i := range resp {
		pipelines[i] = resp[i].toDomain()
	}
	adapterdriven.LogOpDone(ctx, BackendName, "ListPipelines", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return pipelines, nil
}

// GetPipeline gets a single pipeline by ID.
func (r *Repository) GetPipeline(ctx context.Context, pipelineID int64) (*domain.CIPipeline, error) {
	adapterdriven.LogOp(ctx, BackendName, "GetPipeline")
	start := time.Now()

	path := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d", r.encodedProject(), pipelineID)
	var resp glPipeline
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "GetPipeline", err)
		return nil, err
	}
	p := resp.toDomain()
	adapterdriven.LogOpDone(ctx, BackendName, "GetPipeline", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return &p, nil
}

// ListPipelineJobs lists jobs for a pipeline.
func (r *Repository) ListPipelineJobs(ctx context.Context, pipelineID int64) ([]domain.CIJob, error) {
	adapterdriven.LogOp(ctx, BackendName, "ListPipelineJobs")
	start := time.Now()

	path := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d/jobs?per_page=100", r.encodedProject(), pipelineID)
	var resp []glJob
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "ListPipelineJobs", err)
		return nil, err
	}
	jobs := make([]domain.CIJob, len(resp))
	for i := range resp {
		jobs[i] = resp[i].toDomain()
	}
	adapterdriven.LogOpDone(ctx, BackendName, "ListPipelineJobs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return jobs, nil
}

// GetJobLog downloads the trace (log) for a job as plain text.
func (r *Repository) GetJobLog(ctx context.Context, jobID int64) (string, error) {
	adapterdriven.LogOp(ctx, BackendName, "GetJobLog")
	start := time.Now()

	fullURL := fmt.Sprintf("%s/api/v4/projects/%s/jobs/%d/trace", r.baseURL, r.encodedProject(), jobID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("PRIVATE-TOKEN", r.token)

	resp, err := r.client.Do(req)
	if err != nil {
		adapterdriven.LogError(ctx, BackendName, "GetJobLog", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: GET job trace: %d: %s", ErrAPIError, resp.StatusCode, adapterdriven.SanitizeError(string(body)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read trace: %w", err)
	}
	adapterdriven.LogOpDone(ctx, BackendName, "GetJobLog", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return string(data), nil
}

// RetryPipeline retries a pipeline.
func (r *Repository) RetryPipeline(ctx context.Context, pipelineID int64) error {
	if err := r.requireAuth(); err != nil {
		return err
	}
	adapterdriven.LogOp(ctx, BackendName, "RetryPipeline")
	start := time.Now()

	path := fmt.Sprintf("/api/v4/projects/%s/pipelines/%d/retry", r.encodedProject(), pipelineID)
	if err := r.api(ctx, http.MethodPost, path, nil, nil); err != nil {
		adapterdriven.LogError(ctx, BackendName, "RetryPipeline", err)
		return err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "RetryPipeline", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return nil
}

// --- GitLab API response types ---

type glPipeline struct {
	ID        int64   `json:"id"`
	Status    string  `json:"status"`
	Ref       string  `json:"ref"`
	SHA       string  `json:"sha"`
	Source    string  `json:"source"`
	WebURL    string  `json:"web_url"`
	CreatedAt string  `json:"created_at"`
	Duration  float64 `json:"duration"`
}

func (p glPipeline) toDomain() domain.CIPipeline {
	created, _ := time.Parse(time.RFC3339, p.CreatedAt)
	return domain.CIPipeline{
		ID:        p.ID,
		Status:    p.Status,
		Ref:       p.Ref,
		SHA:       p.SHA,
		Source:    p.Source,
		URL:       p.WebURL,
		CreatedAt: created,
		Duration:  int64(p.Duration),
	}
}

type glJob struct {
	ID       int64 `json:"id"`
	Pipeline struct {
		ID int64 `json:"id"`
	} `json:"pipeline"`
	Name      string  `json:"name"`
	Stage     string  `json:"stage"`
	Status    string  `json:"status"`
	WebURL    string  `json:"web_url"`
	StartedAt string  `json:"started_at"`
	Duration  float64 `json:"duration"`
}

func (j glJob) toDomain() domain.CIJob {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	return domain.CIJob{
		ID:         j.ID,
		PipelineID: j.Pipeline.ID,
		Name:       j.Name,
		Stage:      j.Stage,
		Status:     j.Status,
		URL:        j.WebURL,
		StartedAt:  started,
		Duration:   j.Duration,
	}
}
