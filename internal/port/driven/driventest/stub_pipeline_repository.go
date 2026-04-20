package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.PipelineRepository = (*StubPipelineRepository)(nil)

type ListPipelineRunsCall struct {
	JobName string
}

type GetPipelineRunCall struct {
	JobName string
	RunID   string
}

type GetPendingInputsCall struct {
	JobName string
	RunID   string
}

type ApproveInputCall struct {
	JobName string
	RunID   string
}

type AbortInputCall struct {
	JobName string
	RunID   string
}

type GetStageLogCall struct {
	JobName string
	RunID   string
	NodeID  string
}

type StubPipelineRepository struct {
	NameVal        string
	PipelineRuns   []domain.PipelineRun
	PipelineRun    *domain.PipelineRun
	PipelineInputs []domain.PipelineInput
	StageLog       string
	Err            error

	mu                    sync.Mutex
	ListPipelineRunsCalls []ListPipelineRunsCall
	GetPipelineRunCalls   []GetPipelineRunCall
	GetPendingInputsCalls []GetPendingInputsCall
	ApproveInputCalls     []ApproveInputCall
	AbortInputCalls       []AbortInputCall
	GetStageLogCalls      []GetStageLogCall
}

func (s *StubPipelineRepository) Name() string { return s.NameVal }

func (s *StubPipelineRepository) ListPipelineRuns(_ context.Context, jobName string) ([]domain.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListPipelineRunsCalls = append(s.ListPipelineRunsCalls, ListPipelineRunsCall{JobName: jobName})
	return s.PipelineRuns, s.Err
}

func (s *StubPipelineRepository) GetPipelineRun(_ context.Context, jobName, runID string) (*domain.PipelineRun, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetPipelineRunCalls = append(s.GetPipelineRunCalls, GetPipelineRunCall{JobName: jobName, RunID: runID})
	return s.PipelineRun, s.Err
}

func (s *StubPipelineRepository) GetPendingInputs(_ context.Context, jobName, runID string) ([]domain.PipelineInput, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetPendingInputsCalls = append(s.GetPendingInputsCalls, GetPendingInputsCall{JobName: jobName, RunID: runID})
	return s.PipelineInputs, s.Err
}

func (s *StubPipelineRepository) ApproveInput(_ context.Context, jobName, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ApproveInputCalls = append(s.ApproveInputCalls, ApproveInputCall{JobName: jobName, RunID: runID})
	return s.Err
}

func (s *StubPipelineRepository) AbortInput(_ context.Context, jobName, runID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.AbortInputCalls = append(s.AbortInputCalls, AbortInputCall{JobName: jobName, RunID: runID})
	return s.Err
}

func (s *StubPipelineRepository) GetStageLog(_ context.Context, jobName, runID, nodeID string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.GetStageLogCalls = append(s.GetStageLogCalls, GetStageLogCall{JobName: jobName, RunID: runID, NodeID: nodeID})
	return s.StageLog, s.Err
}
