package driventest

import (
	"context"
	"sync"

	"github.com/DanyPops/emcee/internal/domain"
	"github.com/DanyPops/emcee/internal/port/driven"
)

var _ driven.PRRepository = (*StubPRRepository)(nil)

type ListPRsCall struct {
	Filter domain.PRFilter
}

type StubPRRepository struct {
	NameVal string
	PRs     []domain.PullRequest
	Err     error

	mu          sync.Mutex
	ListPRCalls []ListPRsCall
}

func (s *StubPRRepository) Name() string { return s.NameVal }

func (s *StubPRRepository) ListPRs(_ context.Context, filter domain.PRFilter) ([]domain.PullRequest, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ListPRCalls = append(s.ListPRCalls, ListPRsCall{Filter: filter})
	return s.PRs, s.Err
}
