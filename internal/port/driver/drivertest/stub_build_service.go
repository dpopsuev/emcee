package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.BuildService = (*StubBuildService)(nil)

type JobListCall struct {
	Backend string
	Filter  domain.JobFilter
}

type JobGetCall struct {
	Backend string
	Name    string
}

type BuildTriggerCall struct {
	Backend string
	JobName string
	Params  map[string]string
}

type BuildGetCall struct {
	Backend string
	JobName string
	Number  int64
}

type BuildLogCall struct {
	Backend string
	JobName string
	Number  int64
}

type BuildTestResultsCall struct {
	Backend string
	JobName string
	Number  int64
}

type QueueGetCall struct {
	Backend string
}

type StubBuildService struct {
	Jobs        []domain.Job
	Job         *domain.Job
	BuildNumber int64
	Build       *domain.Build
	BuildLog    string
	TestResult  *domain.TestResult
	QueueItems     []domain.QueueItem
	BuildSummaries []domain.BuildSummary
	LastBuild      *domain.Build
	LastSuccessful *domain.Build
	LastFailed     *domain.Build
	JobParameters  []domain.JobParameter
	FolderJobs     []domain.Job
	UpstreamJobs   []domain.Job
	DownstreamJobs []domain.Job
	Err            error

	mu                  sync.Mutex
	ListJobsCalls       []JobListCall
	GetJobCalls         []JobGetCall
	TriggerBuildCalls   []BuildTriggerCall
	GetBuildCalls       []BuildGetCall
	GetBuildLogCalls    []BuildLogCall
	GetTestResultsCalls []BuildTestResultsCall
	GetQueueCalls          []QueueGetCall
	ListBuildsCalls        []BuildListCall
	GetLastBuildCalls      []BuildLastCall
	GetLastSuccessfulCalls []BuildLastCall
	GetLastFailedCalls     []BuildLastCall
	StopBuildCalls         []BuildStopCall
	GetJobParametersCalls  []JobParamsCall
	ListFolderJobsCalls    []FolderJobsCall
	GetUpstreamJobsCalls   []JobDepsCall
	GetDownstreamJobsCalls []JobDepsCall
}

type FolderJobsCall struct {
	Backend    string
	FolderPath string
}

type JobDepsCall struct {
	Backend string
	JobName string
}

type BuildStopCall struct {
	Backend string
	JobName string
	Number  int64
}

type JobParamsCall struct {
	Backend string
	JobName string
}

type BuildListCall struct {
	Backend string
	JobName string
	Limit   int
}

type BuildLastCall struct {
	Backend string
	JobName string
}

func (s *StubBuildService) ListJobs(_ context.Context, backend string, filter domain.JobFilter) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListJobsCalls = append(s.ListJobsCalls, JobListCall{Backend: backend, Filter: filter})
	return s.Jobs, s.Err
}

func (s *StubBuildService) GetJob(_ context.Context, backend, name string) (*domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetJobCalls = append(s.GetJobCalls, JobGetCall{Backend: backend, Name: name})
	return s.Job, s.Err
}

func (s *StubBuildService) TriggerBuild(_ context.Context, backend, jobName string, params map[string]string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TriggerBuildCalls = append(s.TriggerBuildCalls, BuildTriggerCall{Backend: backend, JobName: jobName, Params: params})
	return s.BuildNumber, s.Err
}

func (s *StubBuildService) GetBuild(_ context.Context, backend, jobName string, number int64) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildCalls = append(s.GetBuildCalls, BuildGetCall{Backend: backend, JobName: jobName, Number: number})
	return s.Build, s.Err
}

func (s *StubBuildService) GetBuildLog(_ context.Context, backend, jobName string, number int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildLogCalls = append(s.GetBuildLogCalls, BuildLogCall{Backend: backend, JobName: jobName, Number: number})
	return s.BuildLog, s.Err
}

func (s *StubBuildService) GetTestResults(_ context.Context, backend, jobName string, number int64) (*domain.TestResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetTestResultsCalls = append(s.GetTestResultsCalls, BuildTestResultsCall{Backend: backend, JobName: jobName, Number: number})
	return s.TestResult, s.Err
}

func (s *StubBuildService) GetQueue(_ context.Context, backend string) ([]domain.QueueItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetQueueCalls = append(s.GetQueueCalls, QueueGetCall{Backend: backend})
	return s.QueueItems, s.Err
}

func (s *StubBuildService) ListBuilds(_ context.Context, backend, jobName string, limit int) ([]domain.BuildSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListBuildsCalls = append(s.ListBuildsCalls, BuildListCall{Backend: backend, JobName: jobName, Limit: limit})
	return s.BuildSummaries, s.Err
}

func (s *StubBuildService) GetLastBuild(_ context.Context, backend, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastBuildCalls = append(s.GetLastBuildCalls, BuildLastCall{Backend: backend, JobName: jobName})
	return s.LastBuild, s.Err
}

func (s *StubBuildService) GetLastSuccessfulBuild(_ context.Context, backend, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastSuccessfulCalls = append(s.GetLastSuccessfulCalls, BuildLastCall{Backend: backend, JobName: jobName})
	return s.LastSuccessful, s.Err
}

func (s *StubBuildService) GetLastFailedBuild(_ context.Context, backend, jobName string) (*domain.Build, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetLastFailedCalls = append(s.GetLastFailedCalls, BuildLastCall{Backend: backend, JobName: jobName})
	return s.LastFailed, s.Err
}

func (s *StubBuildService) StopBuild(_ context.Context, backend, jobName string, number int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.StopBuildCalls = append(s.StopBuildCalls, BuildStopCall{Backend: backend, JobName: jobName, Number: number})
	return s.Err
}

func (s *StubBuildService) GetJobParameters(_ context.Context, backend, jobName string) ([]domain.JobParameter, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetJobParametersCalls = append(s.GetJobParametersCalls, JobParamsCall{Backend: backend, JobName: jobName})
	return s.JobParameters, s.Err
}

func (s *StubBuildService) ListFolderJobs(_ context.Context, backend, folderPath string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListFolderJobsCalls = append(s.ListFolderJobsCalls, FolderJobsCall{Backend: backend, FolderPath: folderPath})
	return s.FolderJobs, s.Err
}

func (s *StubBuildService) GetUpstreamJobs(_ context.Context, backend, jobName string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetUpstreamJobsCalls = append(s.GetUpstreamJobsCalls, JobDepsCall{Backend: backend, JobName: jobName})
	return s.UpstreamJobs, s.Err
}

func (s *StubBuildService) GetDownstreamJobs(_ context.Context, backend, jobName string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetDownstreamJobsCalls = append(s.GetDownstreamJobsCalls, JobDepsCall{Backend: backend, JobName: jobName})
	return s.DownstreamJobs, s.Err
}
