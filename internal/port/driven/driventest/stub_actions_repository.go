//nolint:dupl // stubs for separate ISP interfaces share shape but serve different ports
package driventest

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.ActionsRepository = (*StubActionsRepository)(nil)

type StubActionsRepository struct {
	NameVal      string
	WorkflowRuns []domain.WorkflowRun
	WorkflowRun  *domain.WorkflowRun
	Jobs         []domain.WorkflowJob
	Logs         string
	Err          error
}

func (s *StubActionsRepository) Name() string { return s.NameVal }

func (s *StubActionsRepository) ListWorkflowRuns(_ context.Context, _ domain.WorkflowRunFilter) ([]domain.WorkflowRun, error) {
	return s.WorkflowRuns, s.Err
}

func (s *StubActionsRepository) GetWorkflowRun(_ context.Context, _ int64) (*domain.WorkflowRun, error) {
	return s.WorkflowRun, s.Err
}

func (s *StubActionsRepository) ListRunJobs(_ context.Context, _ int64) ([]domain.WorkflowJob, error) {
	return s.Jobs, s.Err
}

func (s *StubActionsRepository) GetRunLogs(_ context.Context, _ int64) (string, error) {
	return s.Logs, s.Err
}

func (s *StubActionsRepository) RerunFailedJobs(_ context.Context, _ int64) error {
	return s.Err
}
