//nolint:dupl // stubs for separate ISP interfaces share shape but serve different ports
package driventest

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.CIRepository = (*StubCIRepository)(nil)

type StubCIRepository struct {
	NameVal   string
	Pipelines []domain.CIPipeline
	Pipeline  *domain.CIPipeline
	Jobs      []domain.CIJob
	JobLog    string
	Err       error
}

func (s *StubCIRepository) Name() string { return s.NameVal }

func (s *StubCIRepository) ListPipelines(_ context.Context, _ domain.CIPipelineFilter) ([]domain.CIPipeline, error) {
	return s.Pipelines, s.Err
}

func (s *StubCIRepository) GetPipeline(_ context.Context, _ int64) (*domain.CIPipeline, error) {
	return s.Pipeline, s.Err
}

func (s *StubCIRepository) ListPipelineJobs(_ context.Context, _ int64) ([]domain.CIJob, error) {
	return s.Jobs, s.Err
}

func (s *StubCIRepository) GetJobLog(_ context.Context, _ int64) (string, error) {
	return s.JobLog, s.Err
}

func (s *StubCIRepository) RetryPipeline(_ context.Context, _ int64) error {
	return s.Err
}
