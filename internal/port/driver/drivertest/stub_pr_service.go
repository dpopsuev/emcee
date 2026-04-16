package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.PRService = (*StubPRService)(nil)

type PRListCall struct {
	Backend string
	Filter  domain.PRFilter
}

type StubPRService struct {
	PRs []domain.PullRequest
	Err error

	mu          sync.Mutex
	ListPRCalls []PRListCall
}

func (s *StubPRService) ListPRs(_ context.Context, backend string, filter domain.PRFilter) ([]domain.PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListPRCalls = append(s.ListPRCalls, PRListCall{Backend: backend, Filter: filter})
	return s.PRs, s.Err
}
