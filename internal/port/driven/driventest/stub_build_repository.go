package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.BuildRepository = (*StubBuildRepository)(nil)

type ListJobsCall struct {
	Filter domain.JobFilter
}

type GetJobCall struct {
	Name string
}

type TriggerBuildCall struct {
	JobName string
	Params  map[string]string
}

type GetBuildCall struct {
	JobName string
	Number  int64
}

type GetBuildLogCall struct {
	JobName string
	Number  int64
}

type GetTestResultsCall struct {
	JobName string
	Number  int64
}

type StubBuildRepository struct {
	NameVal     string
	Jobs        []domain.Job
	Job         *domain.Job
	BuildNumber int64
	Build       *domain.Build
	BuildLog    string
	TestResult  *domain.TestResult
	QueueItems  []domain.QueueItem
	BuildSummaries []domain.BuildSummary
	LastBuild      *domain.Build
	LastSuccessful *domain.Build
	LastFailed     *domain.Build
	JobParameters  []domain.JobParameter
	Err            error

	mu                 sync.Mutex
	ListJobsCalls      []ListJobsCall
	GetJobCalls        []GetJobCall
	TriggerBuildCalls  []TriggerBuildCall
	GetBuildCalls      []GetBuildCall
	GetBuildLogCalls   []GetBuildLogCall
	GetTestResultCalls []GetTestResultsCall
	GetQueueCalls           int
	ListBuildsCalls         []ListBuildsCall
	GetLastBuildCalls       []GetLastBuildCall
	GetLastSuccessfulCalls  []GetLastBuildCall
	GetLastFailedCalls      []GetLastBuildCall
	StopBuildCalls          []StopBuildCall
	GetJobParametersCalls   []GetJobParamsCall
}

type StopBuildCall struct {
	JobName string
	Number  int64
}

type GetJobParamsCall struct {
	JobName string
}

type ListBuildsCall struct {
	JobName string
	Limit   int
}

type GetLastBuildCall struct {
	JobName string
}

func (s *StubBuildRepository) Name() string { return s.NameVal }

func (s *StubBuildRepository) ListJobs(_ context.Context, filter domain.JobFilter) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListJobsCalls = append(s.ListJobsCalls, ListJobsCall{Filter: filter})
	return s.Jobs, s.Err
}

func (s *StubBuildRepository) GetJob(_ context.Context, name string) (*domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetJobCalls = append(s.GetJobCalls, GetJobCall{Name: name})
	return s.Job, s.Err
}

func (s *StubBuildRepository) TriggerBuild(_ context.Context, jobName string, params map[string]string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TriggerBuildCalls = append(s.TriggerBuildCalls, TriggerBuildCall{JobName: jobName, Params: params})
	return s.BuildNumber, s.Err
}

func (s *StubBuildRepository) GetBuild(_ context.Context, jobName string, number int64) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildCalls = append(s.GetBuildCalls, GetBuildCall{JobName: jobName, Number: number})
	return s.Build, s.Err
}

func (s *StubBuildRepository) GetBuildLog(_ context.Context, jobName string, number int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildLogCalls = append(s.GetBuildLogCalls, GetBuildLogCall{JobName: jobName, Number: number})
	return s.BuildLog, s.Err
}

func (s *StubBuildRepository) GetTestResults(_ context.Context, jobName string, number int64) (*domain.TestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetTestResultCalls = append(s.GetTestResultCalls, GetTestResultsCall{JobName: jobName, Number: number})
	return s.TestResult, s.Err
}

func (s *StubBuildRepository) GetQueue(_ context.Context) ([]domain.QueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetQueueCalls++
	return s.QueueItems, s.Err
}

func (s *StubBuildRepository) ListBuilds(_ context.Context, jobName string, limit int) ([]domain.BuildSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListBuildsCalls = append(s.ListBuildsCalls, ListBuildsCall{JobName: jobName, Limit: limit})
	return s.BuildSummaries, s.Err
}

func (s *StubBuildRepository) GetLastBuild(_ context.Context, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastBuildCalls = append(s.GetLastBuildCalls, GetLastBuildCall{JobName: jobName})
	return s.LastBuild, s.Err
}

func (s *StubBuildRepository) GetLastSuccessfulBuild(_ context.Context, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastSuccessfulCalls = append(s.GetLastSuccessfulCalls, GetLastBuildCall{JobName: jobName})
	return s.LastSuccessful, s.Err
}

func (s *StubBuildRepository) GetLastFailedBuild(_ context.Context, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastFailedCalls = append(s.GetLastFailedCalls, GetLastBuildCall{JobName: jobName})
	return s.LastFailed, s.Err
}

func (s *StubBuildRepository) StopBuild(_ context.Context, jobName string, number int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StopBuildCalls = append(s.StopBuildCalls, StopBuildCall{JobName: jobName, Number: number})
	return s.Err
}

func (s *StubBuildRepository) GetJobParameters(_ context.Context, jobName string) ([]domain.JobParameter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetJobParametersCalls = append(s.GetJobParametersCalls, GetJobParamsCall{JobName: jobName})
	return s.JobParameters, s.Err
}
