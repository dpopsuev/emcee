package github

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.ActionsRepository = (*Repository)(nil)

// ListWorkflowRuns lists recent workflow runs.
func (r *Repository) ListWorkflowRuns(ctx context.Context, filter domain.WorkflowRunFilter) ([]domain.WorkflowRun, error) {
	adapterdriven.LogOp(ctx, BackendName, "ListWorkflowRuns")
	start := time.Now()

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}
	rp, err := r.repoPath()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/actions/runs?per_page=%d", rp, limit)
	if filter.Status != "" {
		path += "&status=" + filter.Status
	}
	if filter.Branch != "" {
		path += "&branch=" + filter.Branch
	}

	var resp struct {
		WorkflowRuns []ghWorkflowRun `json:"workflow_runs"`
	}
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "ListWorkflowRuns", err)
		return nil, err
	}
	runs := make([]domain.WorkflowRun, len(resp.WorkflowRuns))
	for i := range resp.WorkflowRuns {
		runs[i] = resp.WorkflowRuns[i].toDomain()
	}
	adapterdriven.LogOpDone(ctx, BackendName, "ListWorkflowRuns", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return runs, nil
}

// GetWorkflowRun gets a single workflow run by ID.
func (r *Repository) GetWorkflowRun(ctx context.Context, runID int64) (*domain.WorkflowRun, error) {
	adapterdriven.LogOp(ctx, BackendName, "GetWorkflowRun")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/actions/runs/%d", rp, runID)
	var resp ghWorkflowRun
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "GetWorkflowRun", err)
		return nil, err
	}
	run := resp.toDomain()
	adapterdriven.LogOpDone(ctx, BackendName, "GetWorkflowRun", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return &run, nil
}

// ListRunJobs lists jobs for a workflow run.
func (r *Repository) ListRunJobs(ctx context.Context, runID int64) ([]domain.WorkflowJob, error) {
	adapterdriven.LogOp(ctx, BackendName, "ListRunJobs")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return nil, err
	}
	path := fmt.Sprintf("%s/actions/runs/%d/jobs", rp, runID)
	var resp struct {
		Jobs []ghWorkflowJob `json:"jobs"`
	}
	if err := r.api(ctx, http.MethodGet, path, nil, &resp); err != nil {
		adapterdriven.LogError(ctx, BackendName, "ListRunJobs", err)
		return nil, err
	}
	jobs := make([]domain.WorkflowJob, len(resp.Jobs))
	for i := range resp.Jobs {
		jobs[i] = resp.Jobs[i].toDomain()
	}
	adapterdriven.LogOpDone(ctx, BackendName, "ListRunJobs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return jobs, nil
}

// GetRunLogs downloads the logs for a workflow run as plain text.
func (r *Repository) GetRunLogs(ctx context.Context, runID int64) (string, error) {
	adapterdriven.LogOp(ctx, BackendName, "GetRunLogs")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s%s/actions/runs/%d/logs", r.baseURL, rp, runID)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, path, http.NoBody)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "token "+r.token)
	req.Header.Set("Accept", "application/vnd.github.v3+json")

	resp, err := r.client.Do(req)
	if err != nil {
		adapterdriven.LogError(ctx, BackendName, "GetRunLogs", err)
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("%w: GET run logs: %d: %s", ErrAPIError, resp.StatusCode, adapterdriven.SanitizeError(string(body)))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	adapterdriven.LogOpDone(ctx, BackendName, "GetRunLogs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return string(data), nil
}

// RerunFailedJobs re-runs all failed jobs in a workflow run.
func (r *Repository) RerunFailedJobs(ctx context.Context, runID int64) error {
	adapterdriven.LogOp(ctx, BackendName, "RerunFailedJobs")
	start := time.Now()

	rp, err := r.repoPath()
	if err != nil {
		return err
	}
	path := fmt.Sprintf("%s/actions/runs/%d/rerun-failed-jobs", rp, runID)
	if err := r.api(ctx, http.MethodPost, path, nil, nil); err != nil {
		adapterdriven.LogError(ctx, BackendName, "RerunFailedJobs", err)
		return err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "RerunFailedJobs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return nil
}

// --- GitHub API response types ---

type ghWorkflowRun struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	HeadBranch string `json:"head_branch"`
	Event      string `json:"event"`
	HTMLURL    string `json:"html_url"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

func (r ghWorkflowRun) toDomain() domain.WorkflowRun {
	created, _ := time.Parse(time.RFC3339, r.CreatedAt)
	updated, _ := time.Parse(time.RFC3339, r.UpdatedAt)
	return domain.WorkflowRun{
		ID:         r.ID,
		Name:       r.Name,
		Status:     r.Status,
		Conclusion: r.Conclusion,
		Branch:     r.HeadBranch,
		Event:      r.Event,
		URL:        r.HTMLURL,
		CreatedAt:  created,
		UpdatedAt:  updated,
	}
}

type ghWorkflowJob struct {
	ID         int64       `json:"id"`
	RunID      int64       `json:"run_id"`
	Name       string      `json:"name"`
	Status     string      `json:"status"`
	Conclusion string      `json:"conclusion"`
	StartedAt  string      `json:"started_at"`
	Steps      []ghJobStep `json:"steps"`
}

func (j ghWorkflowJob) toDomain() domain.WorkflowJob {
	started, _ := time.Parse(time.RFC3339, j.StartedAt)
	steps := make([]domain.JobStep, len(j.Steps))
	for i := range j.Steps {
		steps[i] = domain.JobStep{
			Name:       j.Steps[i].Name,
			Status:     j.Steps[i].Status,
			Conclusion: j.Steps[i].Conclusion,
			Number:     j.Steps[i].Number,
		}
	}
	return domain.WorkflowJob{
		ID:         j.ID,
		RunID:      j.RunID,
		Name:       j.Name,
		Status:     j.Status,
		Conclusion: j.Conclusion,
		StartedAt:  started,
		Steps:      steps,
	}
}

type ghJobStep struct {
	Name       string `json:"name"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
	Number     int    `json:"number"`
}
