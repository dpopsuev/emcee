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
	FolderJobs     []domain.Job
	UpstreamJobs   []domain.Job
	DownstreamJobs []domain.Job
	Artifacts      []domain.BuildArtifact
	BuildRevision  string
	BuildCauses    []domain.BuildCause
	Nodes          []domain.JenkinsNode
	Node           *domain.JenkinsNode
	Views          []domain.JenkinsView
	ViewJobs       []domain.Job
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
	ListFolderJobsCalls     []ListFolderJobsCall
	GetUpstreamJobsCalls    []GetJobDepsCall
	GetDownstreamJobsCalls  []GetJobDepsCall
	ListArtifactsCalls      []GetBuildCall
	GetBuildRevisionCalls   []GetBuildCall
	GetBuildCausesCalls     []GetBuildCall
	ListNodesCalls          int
	GetNodeCalls            []GetNodeCall
	ListViewsCalls          int
	GetViewJobsCalls        []GetViewJobsCall
}

type ListFolderJobsCall struct {
	FolderPath string
}

type GetJobDepsCall struct {
	JobName string
}

type GetNodeCall struct {
	Name string
}

type GetViewJobsCall struct {
	ViewName string
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

func (s *StubBuildRepository) ListFolderJobs(_ context.Context, folderPath string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListFolderJobsCalls = append(s.ListFolderJobsCalls, ListFolderJobsCall{FolderPath: folderPath})
	return s.FolderJobs, s.Err
}

func (s *StubBuildRepository) GetUpstreamJobs(_ context.Context, jobName string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetUpstreamJobsCalls = append(s.GetUpstreamJobsCalls, GetJobDepsCall{JobName: jobName})
	return s.UpstreamJobs, s.Err
}

func (s *StubBuildRepository) GetDownstreamJobs(_ context.Context, jobName string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetDownstreamJobsCalls = append(s.GetDownstreamJobsCalls, GetJobDepsCall{JobName: jobName})
	return s.DownstreamJobs, s.Err
}

func (s *StubBuildRepository) ListArtifacts(_ context.Context, jobName string, number int64) ([]domain.BuildArtifact, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListArtifactsCalls = append(s.ListArtifactsCalls, GetBuildCall{JobName: jobName, Number: number})
	return s.Artifacts, s.Err
}

func (s *StubBuildRepository) GetBuildRevision(_ context.Context, jobName string, number int64) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildRevisionCalls = append(s.GetBuildRevisionCalls, GetBuildCall{JobName: jobName, Number: number})
	return s.BuildRevision, s.Err
}

func (s *StubBuildRepository) GetBuildCauses(_ context.Context, jobName string, number int64) ([]domain.BuildCause, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetBuildCausesCalls = append(s.GetBuildCausesCalls, GetBuildCall{JobName: jobName, Number: number})
	return s.BuildCauses, s.Err
}

func (s *StubBuildRepository) ListNodes(_ context.Context) ([]domain.JenkinsNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListNodesCalls++
	return s.Nodes, s.Err
}

func (s *StubBuildRepository) GetNode(_ context.Context, name string) (*domain.JenkinsNode, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetNodeCalls = append(s.GetNodeCalls, GetNodeCall{Name: name})
	return s.Node, s.Err
}

func (s *StubBuildRepository) ListViews(_ context.Context) ([]domain.JenkinsView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListViewsCalls++
	return s.Views, s.Err
}

func (s *StubBuildRepository) GetViewJobs(_ context.Context, viewName string) ([]domain.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetViewJobsCalls = append(s.GetViewJobsCalls, GetViewJobsCall{ViewName: viewName})
	return s.ViewJobs, s.Err
}
