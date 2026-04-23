package drivertest

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/port/driver"
)

var _ driver.BulkService = (*StubBulkService)(nil)

type BulkCreateCall struct {
	Backend string
	Inputs  []domain.CreateInput
}

type BulkUpdateCall struct {
	Backend string
	Inputs  []domain.BulkUpdateInput
}

type StubBulkService struct {
	CreateResult *domain.BulkCreateResult
	UpdateResult *domain.BulkUpdateResult
	Err          error

	mu              sync.Mutex
	BulkCreateCalls []BulkCreateCall
	BulkUpdateCalls []BulkUpdateCall
}

func (s *StubBulkService) BulkCreateIssues(_ context.Context, backend string, inputs []domain.CreateInput) (*domain.BulkCreateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BulkCreateCalls = append(s.BulkCreateCalls, BulkCreateCall{Backend: backend, Inputs: inputs})
	return s.CreateResult, s.Err
}

func (s *StubBulkService) BulkUpdateIssues(_ context.Context, backend string, inputs []domain.BulkUpdateInput) (*domain.BulkUpdateResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.BulkUpdateCalls = append(s.BulkUpdateCalls, BulkUpdateCall{Backend: backend, Inputs: inputs})
	return s.UpdateResult, s.Err
}
