//nolint:dupl // stubs for separate ISP interfaces share shape but serve different ports
package drivertest

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.ActionsService = (*StubActionsService)(nil)

type StubActionsService struct {
	WorkflowRuns []domain.WorkflowRun
	WorkflowRun  *domain.WorkflowRun
	RunJobs      []domain.WorkflowJob
	RunLogs      string
	Err          error
}

func (s *StubActionsService) ListWorkflowRuns(_ context.Context, _ string, _ domain.WorkflowRunFilter) ([]domain.WorkflowRun, error) {
	return s.WorkflowRuns, s.Err
}

func (s *StubActionsService) GetWorkflowRun(_ context.Context, _ string, _ int64) (*domain.WorkflowRun, error) {
	return s.WorkflowRun, s.Err
}

func (s *StubActionsService) ListRunJobs(_ context.Context, _ string, _ int64) ([]domain.WorkflowJob, error) {
	return s.RunJobs, s.Err
}

func (s *StubActionsService) GetRunLogs(_ context.Context, _ string, _ int64) (string, error) {
	return s.RunLogs, s.Err
}

func (s *StubActionsService) RerunFailedJobs(_ context.Context, _ string, _ int64) error {
	return s.Err
}
