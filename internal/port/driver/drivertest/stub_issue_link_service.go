package drivertest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driver"
)

var _ driver.IssueLinkService = (*StubIssueLinkService)(nil)

type StubIssueLinkService struct {
	Err error

	mu             sync.Mutex
	LinkIssueCalls []domain.IssueLinkInput
}

func (s *StubIssueLinkService) LinkIssue(_ context.Context, _ string, input domain.IssueLinkInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LinkIssueCalls = append(s.LinkIssueCalls, input)
	return s.Err
}
