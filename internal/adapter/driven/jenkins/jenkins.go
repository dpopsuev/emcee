// Package jenkins implements the driven (outbound) adapter for Jenkins CI via bndr/gojenkins.
package jenkins

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

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
	jenkins *gojenkins.Jenkins
	baseURL string
}

// New creates a Jenkins repository.
func New(ctx context.Context, baseURL, user, token string) (*Repository, error) {
	j, err := gojenkins.CreateJenkins(nil, baseURL, user, token).Init(ctx)
	if err != nil {
		return nil, fmt.Errorf("jenkins init: %w", err)
	}
	return &Repository{jenkins: j, baseURL: baseURL}, nil
}

func (r *Repository) Name() string { return BackendName }

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

// --- BuildRepository implementation ---

func (r *Repository) ListJobs(ctx context.Context, filter domain.JobFilter) ([]domain.Job, error) {
	adapterdriven.LogOp(ctx, BackendName, "list_jobs")
	inner, err := r.jenkins.GetAllJobs(ctx)
	if err != nil {
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
	return jobs, nil
}

func (r *Repository) GetJob(ctx context.Context, name string) (*domain.Job, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_job", slog.String(adapterdriven.LogKeyID, name))
	j, err := r.jenkins.GetJob(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrJobNotFound, name)
	}
	return &domain.Job{
		Name:      j.GetName(),
		URL:       j.Raw.URL,
		Color:     j.Raw.Color,
		Buildable: j.Raw.Buildable,
		InQueue:   j.Raw.InQueue,
	}, nil
}

func (r *Repository) TriggerBuild(ctx context.Context, jobName string, params map[string]string) (int64, error) {
	adapterdriven.LogWrite(ctx, BackendName, "trigger_build", slog.String(adapterdriven.LogKeyID, jobName))
	queueID, err := r.jenkins.BuildJob(ctx, jobName, params)
	if err != nil {
		return 0, err
	}
	return queueID, nil
}

func (r *Repository) GetBuild(ctx context.Context, jobName string, number int64) (*domain.Build, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_build",
		slog.String(adapterdriven.LogKeyID, jobName),
		slog.Int64("number", number))
	build, err := r.jenkins.GetBuild(ctx, jobName, number)
	if err != nil {
		return nil, fmt.Errorf("%w: %s #%d", ErrBuildNotFound, jobName, number)
	}
	return &domain.Build{
		Number:    build.GetBuildNumber(),
		Result:    domain.BuildResult(build.GetResult()),
		Building:  build.IsRunning(ctx),
		Duration:  int64(build.GetDuration()),
		Timestamp: build.GetTimestamp(),
		URL:       build.GetUrl(),
	}, nil
}

func (r *Repository) GetBuildLog(ctx context.Context, jobName string, number int64) (string, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_build_log",
		slog.String(adapterdriven.LogKeyID, jobName),
		slog.Int64("number", number))
	build, err := r.jenkins.GetBuild(ctx, jobName, number)
	if err != nil {
		return "", fmt.Errorf("%w: %s #%d", ErrBuildNotFound, jobName, number)
	}
	return build.GetConsoleOutput(ctx), nil
}

func (r *Repository) GetTestResults(ctx context.Context, jobName string, number int64) (*domain.TestResult, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_test_results",
		slog.String(adapterdriven.LogKeyID, jobName),
		slog.Int64("number", number))
	build, err := r.jenkins.GetBuild(ctx, jobName, number)
	if err != nil {
		return nil, fmt.Errorf("%w: %s #%d", ErrBuildNotFound, jobName, number)
	}
	testResult, err := build.GetResultSet(ctx)
	if err != nil {
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
	return result, nil
}

func (r *Repository) GetQueue(ctx context.Context) ([]domain.QueueItem, error) {
	adapterdriven.LogOp(ctx, BackendName, "get_queue")
	queue, err := r.jenkins.GetQueue(ctx)
	if err != nil {
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
	return items, nil
}
