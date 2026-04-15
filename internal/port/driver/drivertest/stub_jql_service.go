package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.JQLService = (*StubJQLService)(nil)

type JQLSearchCall struct {
	Backend string
	JQL     string
	Limit   int
}

type StubJQLService struct {
	Issues []domain.Issue
	Err    error

	mu             sync.Mutex
	SearchJQLCalls []JQLSearchCall
}

func (s *StubJQLService) SearchJQL(_ context.Context, backend, jql string, limit int) ([]domain.Issue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.SearchJQLCalls = append(s.SearchJQLCalls, JQLSearchCall{Backend: backend, JQL: jql, Limit: limit})
	return s.Issues, s.Err
}
