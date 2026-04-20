// Package jenkins implements the driven (outbound) adapter for Jenkins CI via bndr/gojenkins.
package jenkins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	adapterdriven "github.com/DanyPops/emcee/internal/adapter/driven"
	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
	"github.com/bndr/gojenkins"
)

const (
	BackendName  = "jenkins"
	defaultLimit = 50
)

var (
	ErrJobNotFound     = errors.New("job not found")
	ErrBuildNotFound   = errors.New("build not found")
	ErrNotIssueBackend = errors.New("jenkins is not an issue backend — use jobs/builds actions")
)

// Compile-time interface compliance checks.
var (
	_ driven.IssueRepository = (*Repository)(nil)
	_ driven.BuildRepository = (*Repository)(nil)
)

// Repository implements driven.BuildRepository for Jenkins.
type Repository struct {
	name    string
	jenkins *gojenkins.Jenkins
	baseURL string
}

// New creates a Jenkins repository.
func New(ctx context.Context, name, baseURL, user, token string) (*Repository, error) {
	j, err := gojenkins.CreateJenkins(nil, baseURL, user, token).Init(ctx)
	if err != nil {
		return nil, fmt.Errorf("jenkins init: %w", err)
	}
	return &Repository{name: name, jenkins: j, baseURL: baseURL}, nil
}

func (r *Repository) Name() string { return r.name }

// --- IssueRepository stubs (Jenkins is not an issue backend) ---

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

// --- helpers ---

func (r *Repository) logErr(ctx context.Context, op string, err error) {
	adapterdriven.LogError(ctx, BackendName, op, err)
}

func (r *Repository) mapBuild(ctx context.Context, b *gojenkins.Build) *domain.Build {
	return &domain.Build{
		Number:    b.GetBuildNumber(),
		Result:    domain.BuildResult(b.GetResult()),
		Building:  b.IsRunning(ctx),
		Duration:  int64(b.GetDuration()),
		Timestamp: b.GetTimestamp(),
		URL:       b.GetUrl(),
	}
}

func (r *Repository) getJob(ctx context.Context, name string) (*gojenkins.Job, error) {
	j, err := r.jenkins.GetJob(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	return j, nil
}

func (r *Repository) getBuild(ctx context.Context, jobName string, number int64) (*gojenkins.Build, error) {
	b, err := r.jenkins.GetBuild(ctx, jobName, number)
	if err != nil {
		return nil, fmt.Errorf("%w: %s #%d", ErrBuildNotFound, jobName, number)
	}
	return b, nil
}

// --- BuildRepository implementation ---

func (r *Repository) ListJobs(ctx context.Context, filter domain.JobFilter) ([]domain.Job, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "list_jobs")
	inner, err := r.jenkins.GetAllJobs(ctx)
	if err != nil {
		r.logErr(ctx, "list_jobs", err)
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	jobs := make([]domain.Job, 0, len(inner))
	for i, j := range inner {
		if i >= limit {
			break
		}
		jobs = append(jobs, domain.Job{
			Name:      j.GetName(),
			URL:       j.Raw.URL,
			Color:     j.Raw.Color,
			Buildable: j.Raw.Buildable,
			InQueue:   j.Raw.InQueue,
		})
	}
	adapterdriven.LogOpDone(ctx, BackendName, "list_jobs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(jobs)))
	return jobs, nil
}

func (r *Repository) GetJob(ctx context.Context, name string) (*domain.Job, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_job", slog.String(adapterdriven.LogKeyID, name))
	j, err := r.getJob(ctx, name)
	if err != nil {
		r.logErr(ctx, "get_job", err)
		return nil, err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "get_job", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return &domain.Job{
		Name:      j.GetName(),
		URL:       j.Raw.URL,
		Color:     j.Raw.Color,
		Buildable: j.Raw.Buildable,
		InQueue:   j.Raw.InQueue,
	}, nil
}

func (r *Repository) TriggerBuild(ctx context.Context, jobName string, params map[string]string) (int64, error) {
	start := time.Now()
	adapterdriven.LogWrite(ctx, BackendName, "trigger_build", slog.String(adapterdriven.LogKeyID, jobName))
	queueID, err := r.jenkins.BuildJob(ctx, jobName, params)
	if err != nil {
		r.logErr(ctx, "trigger_build", err)
		return 0, err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "trigger_build", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int64("queue_id", queueID))
	return queueID, nil
}

func (r *Repository) GetBuild(ctx context.Context, jobName string, number int64) (*domain.Build, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_build", slog.String(adapterdriven.LogKeyID, jobName), slog.Int64("number", number))
	b, err := r.getBuild(ctx, jobName, number)
	if err != nil {
		r.logErr(ctx, "get_build", err)
		return nil, err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "get_build", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return r.mapBuild(ctx, b), nil
}

func (r *Repository) GetBuildLog(ctx context.Context, jobName string, number int64) (string, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_build_log", slog.String(adapterdriven.LogKeyID, jobName), slog.Int64("number", number))
	b, err := r.getBuild(ctx, jobName, number)
	if err != nil {
		r.logErr(ctx, "get_build_log", err)
		return "", err
	}
	output := b.GetConsoleOutput(ctx)
	adapterdriven.LogOpDone(ctx, BackendName, "get_build_log", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return output, nil
}

func (r *Repository) GetTestResults(ctx context.Context, jobName string, number int64) (*domain.TestResult, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_test_results", slog.String(adapterdriven.LogKeyID, jobName), slog.Int64("number", number))
	b, err := r.getBuild(ctx, jobName, number)
	if err != nil {
		r.logErr(ctx, "get_test_results", err)
		return nil, err
	}
	testResult, err := b.GetResultSet(ctx)
	if err != nil {
		r.logErr(ctx, "get_test_results", err)
		return nil, fmt.Errorf("test results: %w", err)
	}
	result := &domain.TestResult{
		Passed:   int(testResult.PassCount),
		Failed:   int(testResult.FailCount),
		Skipped:  int(testResult.SkipCount),
		Duration: testResult.Duration,
	}
	result.Total = result.Passed + result.Failed + result.Skipped

	for i := range testResult.Suites {
		s := &testResult.Suites[i]
		suite := domain.TestSuite{
			Name:     s.Name,
			Duration: s.Duration,
		}
		for j := range s.Cases {
			suite.Cases = append(suite.Cases, domain.TestCase{
				Name:   s.Cases[j].Name,
				Status: s.Cases[j].Status,
			})
		}
		result.Suites = append(result.Suites, suite)
	}
	adapterdriven.LogOpDone(ctx, BackendName, "get_test_results", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return result, nil
}

func (r *Repository) GetQueue(ctx context.Context) ([]domain.QueueItem, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_queue")
	queue, err := r.jenkins.GetQueue(ctx)
	if err != nil {
		r.logErr(ctx, "get_queue", err)
		return nil, err
	}
	items := make([]domain.QueueItem, 0, len(queue.Raw.Items))
	for i := range queue.Raw.Items {
		items = append(items, domain.QueueItem{
			ID:        queue.Raw.Items[i].ID,
			Why:       queue.Raw.Items[i].Why,
			Blocked:   queue.Raw.Items[i].Blocked,
			Buildable: queue.Raw.Items[i].Buildable,
			TaskName:  queue.Raw.Items[i].Task.Name,
		})
	}
	adapterdriven.LogOpDone(ctx, BackendName, "get_queue", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(items)))
	return items, nil
}

// --- Build history ---

func (r *Repository) ListBuilds(ctx context.Context, jobName string, limit int) ([]domain.BuildSummary, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "list_builds", slog.String(adapterdriven.LogKeyID, jobName))
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		r.logErr(ctx, "list_builds", err)
		return nil, err
	}
	ids, err := j.GetAllBuildIds(ctx)
	if err != nil {
		r.logErr(ctx, "list_builds", err)
		return nil, err
	}
	if limit <= 0 {
		limit = defaultLimit
	}
	builds := make([]domain.BuildSummary, 0, len(ids))
	for i, id := range ids {
		if i >= limit {
			break
		}
		builds = append(builds, domain.BuildSummary{
			Number: id.Number,
			URL:    id.URL,
		})
	}
	adapterdriven.LogOpDone(ctx, BackendName, "list_builds", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(builds)))
	return builds, nil
}

// getLastBuildByType extracts the common pattern for GetLastBuild/GetLastSuccessful/GetLastFailed.
func (r *Repository) getLastBuildByType(ctx context.Context, jobName, op, label string, fn func(context.Context) (*gojenkins.Build, error)) (*domain.Build, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, op, slog.String(adapterdriven.LogKeyID, jobName))
	b, err := fn(ctx)
	if err != nil {
		r.logErr(ctx, op, err)
		return nil, fmt.Errorf("%w: %s (%s)", ErrBuildNotFound, jobName, label)
	}
	adapterdriven.LogOpDone(ctx, BackendName, op, slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return r.mapBuild(ctx, b), nil
}

func (r *Repository) GetLastBuild(ctx context.Context, jobName string) (*domain.Build, error) {
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		return nil, err
	}
	return r.getLastBuildByType(ctx, jobName, "get_last_build", "last", j.GetLastBuild)
}

func (r *Repository) GetLastSuccessfulBuild(ctx context.Context, jobName string) (*domain.Build, error) {
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		return nil, err
	}
	return r.getLastBuildByType(ctx, jobName, "get_last_successful_build", "last successful", j.GetLastSuccessfulBuild)
}

func (r *Repository) GetLastFailedBuild(ctx context.Context, jobName string) (*domain.Build, error) {
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		return nil, err
	}
	return r.getLastBuildByType(ctx, jobName, "get_last_failed_build", "last failed", j.GetLastFailedBuild)
}

// --- Build control ---

func (r *Repository) StopBuild(ctx context.Context, jobName string, number int64) error {
	start := time.Now()
	adapterdriven.LogWrite(ctx, BackendName, "stop_build", slog.String(adapterdriven.LogKeyID, jobName), slog.Int64("number", number))
	b, err := r.getBuild(ctx, jobName, number)
	if err != nil {
		r.logErr(ctx, "stop_build", err)
		return err
	}
	if _, err := b.Stop(ctx); err != nil {
		r.logErr(ctx, "stop_build", err)
		return err
	}
	adapterdriven.LogOpDone(ctx, BackendName, "stop_build", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)))
	return nil
}

func (r *Repository) GetJobParameters(ctx context.Context, jobName string) ([]domain.JobParameter, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "get_job_params", slog.String(adapterdriven.LogKeyID, jobName))
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		r.logErr(ctx, "get_job_params", err)
		return nil, err
	}
	defs, err := j.GetParameters(ctx)
	if err != nil {
		r.logErr(ctx, "get_job_params", err)
		return nil, err
	}
	params := make([]domain.JobParameter, 0, len(defs))
	for i := range defs {
		params = append(params, domain.JobParameter{
			Name:         defs[i].Name,
			Type:         defs[i].Type,
			DefaultValue: fmt.Sprintf("%v", defs[i].DefaultParameterValue.Value),
			Description:  defs[i].Description,
		})
	}
	adapterdriven.LogOpDone(ctx, BackendName, "get_job_params", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(params)))
	return params, nil
}

// --- Folder navigation ---

func mapInnerJobs(inner []gojenkins.InnerJob) []domain.Job {
	jobs := make([]domain.Job, 0, len(inner))
	for i := range inner {
		jobs = append(jobs, domain.Job{
			Name:  inner[i].Name,
			URL:   inner[i].Url,
			Color: inner[i].Color,
		})
	}
	return jobs
}

func (r *Repository) ListFolderJobs(ctx context.Context, folderPath string) ([]domain.Job, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, "list_folder_jobs", slog.String(adapterdriven.LogKeyID, folderPath))
	folder, err := r.jenkins.GetFolder(ctx, folderPath)
	if err != nil {
		r.logErr(ctx, "list_folder_jobs", err)
		return nil, fmt.Errorf("%w: folder %s", ErrJobNotFound, folderPath)
	}
	jobs := mapInnerJobs(folder.Raw.Jobs)
	adapterdriven.LogOpDone(ctx, BackendName, "list_folder_jobs", slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(jobs)))
	return jobs, nil
}

func (r *Repository) getJobDeps(ctx context.Context, jobName, op string, metadataFn func() []gojenkins.InnerJob) ([]domain.Job, error) {
	start := time.Now()
	adapterdriven.LogOp(ctx, BackendName, op, slog.String(adapterdriven.LogKeyID, jobName))
	jobs := mapInnerJobs(metadataFn())
	adapterdriven.LogOpDone(ctx, BackendName, op, slog.Duration(adapterdriven.LogKeyElapsed, time.Since(start)), slog.Int(adapterdriven.LogKeyCount, len(jobs)))
	return jobs, nil
}

func (r *Repository) GetUpstreamJobs(ctx context.Context, jobName string) ([]domain.Job, error) {
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		return nil, err
	}
	return r.getJobDeps(ctx, jobName, "get_upstream_jobs", j.GetUpstreamJobsMetadata)
}

func (r *Repository) GetDownstreamJobs(ctx context.Context, jobName string) ([]domain.Job, error) {
	j, err := r.getJob(ctx, jobName)
	if err != nil {
		return nil, err
	}
	return r.getJobDeps(ctx, jobName, "get_downstream_jobs", j.GetDownstreamJobsMetadata)
}
