package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.PipelineService = (*StubPipelineService)(nil)

type PipelineRunsListCall struct {
	Backend string
	JobName string
}

type PipelineRunGetCall struct {
	Backend string
	JobName string
	RunID   string
}

type PendingInputsCall struct {
	Backend string
	JobName string
	RunID   string
}

type PipelineApproveCall struct {
	Backend string
	JobName string
	RunID   string
}

type PipelineAbortCall struct {
	Backend string
	JobName string
	RunID   string
}

type PipelineStageLogCall struct {
	Backend string
	JobName string
	RunID   string
	NodeID  string
}

type StubPipelineService struct {
	PipelineRuns   []domain.PipelineRun
	PipelineRun    *domain.PipelineRun
	PipelineInputs []domain.PipelineInput
	StageLog       string
	Err            error

	mu                    sync.Mutex
	ListPipelineRunsCalls []PipelineRunsListCall
	GetPipelineRunCalls   []PipelineRunGetCall
	GetPendingInputsCalls []PendingInputsCall
	ApproveInputCalls     []PipelineApproveCall
	AbortInputCalls       []PipelineAbortCall
	GetStageLogCalls      []PipelineStageLogCall
}

func (s *StubPipelineService) ListPipelineRuns(_ context.Context, backend, jobName string) ([]domain.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListPipelineRunsCalls = append(s.ListPipelineRunsCalls, PipelineRunsListCall{Backend: backend, JobName: jobName})
	return s.PipelineRuns, s.Err
}

func (s *StubPipelineService) GetPipelineRun(_ context.Context, backend, jobName, runID string) (*domain.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetPipelineRunCalls = append(s.GetPipelineRunCalls, PipelineRunGetCall{Backend: backend, JobName: jobName, RunID: runID})
	return s.PipelineRun, s.Err
}

func (s *StubPipelineService) GetPendingInputs(_ context.Context, backend, jobName, runID string) ([]domain.PipelineInput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetPendingInputsCalls = append(s.GetPendingInputsCalls, PendingInputsCall{Backend: backend, JobName: jobName, RunID: runID})
	return s.PipelineInputs, s.Err
}

func (s *StubPipelineService) ApproveInput(_ context.Context, backend, jobName, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ApproveInputCalls = append(s.ApproveInputCalls, PipelineApproveCall{Backend: backend, JobName: jobName, RunID: runID})
	return s.Err
}

func (s *StubPipelineService) AbortInput(_ context.Context, backend, jobName, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AbortInputCalls = append(s.AbortInputCalls, PipelineAbortCall{Backend: backend, JobName: jobName, RunID: runID})
	return s.Err
}

func (s *StubPipelineService) GetStageLog(_ context.Context, backend, jobName, runID, nodeID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetStageLogCalls = append(s.GetStageLogCalls, PipelineStageLogCall{Backend: backend, JobName: jobName, RunID: runID, NodeID: nodeID})
	return s.StageLog, s.Err
}
