//nolint:dupl // stubs for separate ISP interfaces share shape but serve different ports
package drivertest

import (
	"context"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.CIService = (*StubCIService)(nil)

type StubCIService struct {
	Pipelines    []domain.CIPipeline
	Pipeline     *domain.CIPipeline
	PipelineJobs []domain.CIJob
	JobLogText   string
	Err          error
}

func (s *StubCIService) ListPipelines(_ context.Context, _ string, _ domain.CIPipelineFilter) ([]domain.CIPipeline, error) {
	return s.Pipelines, s.Err
}

func (s *StubCIService) GetPipeline(_ context.Context, _ string, _ int64) (*domain.CIPipeline, error) {
	return s.Pipeline, s.Err
}

func (s *StubCIService) ListPipelineJobs(_ context.Context, _ string, _ int64) ([]domain.CIJob, error) {
	return s.PipelineJobs, s.Err
}

func (s *StubCIService) GetJobLog(_ context.Context, _ string, _ int64) (string, error) {
	return s.JobLogText, s.Err
}

func (s *StubCIService) RetryPipeline(_ context.Context, _ string, _ int64) error {
	return s.Err
}
