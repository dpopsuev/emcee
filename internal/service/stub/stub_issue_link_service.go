package stub

import (
	"context"
	"sync"

	"github.com/dpopsuev/emcee/internal/domain"
	"github.com/dpopsuev/emcee/internal/service"
)

var _ service.IssueLinkService = (*StubIssueLinkService)(nil)

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

func (s *StubIssueLinkService) UnlinkIssue(_ context.Context, _, _, _, _ string) error {
	return s.Err
}

func (s *StubIssueLinkService) ListLinkTypes(_ context.Context, _ string) ([]domain.IssueLinkType, error) {
	return nil, s.Err
}
